package main

import (
	"context"
	"crypto/tls"
	"expvar"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/handlers"
	datastore "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/pkg/errors"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"
	"go.uber.org/zap"

	"github.com/consensus-shipyard/calibration/faucet/internal/data"
	"github.com/consensus-shipyard/calibration/faucet/internal/faucet"
	app "github.com/consensus-shipyard/calibration/faucet/internal/http"
)

var build = "develop"

func main() {
	logger := logging.Logger("CALIBRATION-FAUCET")

	lvl, err := logging.LevelFromString("info")
	if err != nil {
		panic(err)
	}
	logging.SetAllLoggers(lvl)

	if err := run(logger); err != nil {
		logger.Fatalln("main: error:", err)
	}
}

func run(log *logging.ZapEventLogger) error {
	// =========================================================================
	// Configuration

	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:60s"`
			IdleTimeout     time.Duration `conf:"default:120s"`
			ShutdownTimeout time.Duration `conf:"default:20s"`
			Host            string        `conf:"default:0.0.0.0:8000"`
			BackendHost     string        `conf:"required"`
			AllowedOrigins  []string      `conf:"required"`
		}
		TLS struct {
			Disable  bool   `conf:"default:true"`
			CertFile string `conf:"default:nocert.pem"`
			KeyFile  string `conf:"default:nokey.pem"`
		}
		Faucet struct {
			// Amount of tokens that below is in FIL.
			TotalWithdrawalLimit   uint64 `conf:"default:10000"`
			AddressWithdrawalLimit uint64 `conf:"default:20"`
			WithdrawalAmount       uint64 `conf:"default:10"`
		}
		Ethereum struct {
			APIHost    string `conf:"default:https://mainnet.infura.io"`
			APIToken   string
			PrivateKey string
		}
		DB struct {
			Path     string `conf:"default:./_db_data"`
			Readonly bool   `conf:"default:false"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "Calibration Faucet Service",
		},
	}

	const prefix = "FAUCET"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return err
	}

	// =========================================================================
	// App Starting

	ctx := context.Background()

	log.Infow("starting service", "version", build)
	defer log.Infow("shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Infow("startup", "config", out)

	expvar.NewString("build").Set(build)

	// =========================================================================
	// Database Support

	log.Infow("startup", "status", "initializing database support", "path", cfg.DB.Path)

	db, err := datastore.NewDatastore(cfg.DB.Path, &datastore.Options{
		Compression: ldbopts.NoCompression,
		NoSync:      false,
		Strict:      ldbopts.StrictAll,
		ReadOnly:    cfg.DB.Readonly,
	})
	if err != nil {
		return fmt.Errorf("couldn't initialize leveldb database: %w", err)
	}

	defer func() {
		log.Infow("shutdown", "status", "stopping leveldb")
		err = db.Close()
		if err != nil {
			log.Errorf("closing DB error: %s", err)
		}
	}()

	// =========================================================================
	// Start Ethereum client

	client, err := ethclient.Dial(cfg.Ethereum.APIHost)
	if err != nil {
		return fmt.Errorf("failed to connect to API: %w", err)
	}

	account, err := data.NewAccount(cfg.Ethereum.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to initialize account: %w", err)
	}

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize private key: %w", err)
	}

	// =========================================================================
	// Start API Service

	log.Infow("startup", "status", "initializing HTTP API support")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	var tlsConfig *tls.Config
	if !cfg.TLS.Disable {
		cert, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS key pair: %w", err)
		}
		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
	}

	api := http.Server{
		TLSConfig: tlsConfig,
		Addr:      cfg.Web.Host,
		Handler: handlers.RecoveryHandler()(app.FaucetHandler(log, client, db, build, &faucet.Config{
			AllowedOrigins:         cfg.Web.AllowedOrigins,
			BackendAddress:         cfg.Web.BackendHost,
			TotalWithdrawalLimit:   cfg.Faucet.TotalWithdrawalLimit,
			AddressWithdrawalLimit: cfg.Faucet.AddressWithdrawalLimit,
			WithdrawalAmount:       cfg.Faucet.WithdrawalAmount,
			Account:                account,
			ChainID:                chainID,
		})),
		ReadTimeout:  cfg.Web.ReadTimeout,
		WriteTimeout: cfg.Web.WriteTimeout,
		IdleTimeout:  cfg.Web.IdleTimeout,
		ErrorLog:     zap.NewStdLog(log.Desugar()),
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Infow("startup", "status", "api router started", "host", api.Addr)
		switch cfg.TLS.Disable {
		case true:
			serverErrors <- api.ListenAndServe()
		case false:
			serverErrors <- api.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		}
	}()

	// =========================================================================
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Infow("shutdown", "status", "shutdown started", "signal", sig)
		defer log.Infow("shutdown", "status", "shutdown complete", "signal", sig)

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

		if err := api.Shutdown(ctx); err != nil {
			if err := api.Close(); err != nil {
				log.Errorw("shutdown", "status", "api shutdown", "err", err)
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}
	return nil
}
