package http

import (
	"net/http"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/mux"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/rs/cors"

	"github.com/consensus-shipyard/calibration/faucet/internal/faucet"
)

func FaucetHandler(logger *logging.ZapEventLogger, client *ethclient.Client, db datastore.Batching, build string, cfg *faucet.Config) http.Handler {
	h := NewHealth(logger, client, build)
	faucetService := faucet.NewService(logger, client, db, cfg)
	srv := NewWebService(logger, faucetService, cfg.BackendAddress)

	r := mux.NewRouter().StrictSlash(true)

	r.HandleFunc("/readiness", h.Readiness).Methods("GET")
	r.HandleFunc("/liveness", h.Liveness).Methods("GET")
	r.HandleFunc("/fund", srv.handleFunds).Methods("POST")
	r.HandleFunc("/", srv.handleHome)
	r.HandleFunc("/js/scripts.js", srv.handleScript)
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./static"))))

	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowCredentials: true,
	})

	return c.Handler(r)
}
