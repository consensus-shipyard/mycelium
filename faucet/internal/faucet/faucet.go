package faucet

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"

	"github.com/consensus-shipyard/calibration/faucet/internal/data"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/consensus-shipyard/calibration/faucet/internal/db"
)

var (
	ErrExceedTotalAllowedFunds = fmt.Errorf("transaction exceeds total allowed funds per day")
	ErrExceedAddrAllowedFunds  = fmt.Errorf("transaction to exceeds daily allowed funds per address")
)

type PushWaiter interface {
	SendTransaction(ctx context.Context, tx *types.Transaction) error
}

type Config struct {
	AllowedOrigins         []string
	TotalWithdrawalLimit   uint64
	AddressWithdrawalLimit uint64
	WithdrawalAmount       uint64
	BackendAddress         string
	Account                *data.EthereumAccount
	ChainID                *big.Int
}

type Service struct {
	log    *logging.ZapEventLogger
	client *ethclient.Client
	db     *db.Database
	cfg    *Config
}

func NewService(log *logging.ZapEventLogger, client *ethclient.Client, store datastore.Datastore, cfg *Config) *Service {
	return &Service{
		cfg:    cfg,
		log:    log,
		client: client,
		db:     db.NewDatabase(store),
	}
}

func (s *Service) FundAddress(ctx context.Context, targetAddr common.Address) error {
	addrInfo, err := s.db.GetAddrInfo(ctx, targetAddr)
	if err != nil {
		return err
	}
	s.log.Infof("target address info: %v", addrInfo)

	totalInfo, err := s.db.GetTotalInfo(ctx)
	if err != nil {
		return err
	}
	s.log.Infof("total info: %v", totalInfo)

	if addrInfo.LatestWithdrawal.IsZero() || time.Since(addrInfo.LatestWithdrawal) >= 24*time.Hour {
		addrInfo.Amount = 0
		addrInfo.LatestWithdrawal = time.Now()
	}

	if totalInfo.LatestWithdrawal.IsZero() || time.Since(totalInfo.LatestWithdrawal) >= 24*time.Hour {
		totalInfo.Amount = 0
		totalInfo.LatestWithdrawal = time.Now()
	}

	if totalInfo.Amount >= s.cfg.TotalWithdrawalLimit {
		return ErrExceedTotalAllowedFunds
	}

	if addrInfo.Amount >= s.cfg.AddressWithdrawalLimit {
		return ErrExceedAddrAllowedFunds
	}

	s.log.Infof("funding %v is allowed", targetAddr)

	if err = s.transferETH(ctx, targetAddr); err != nil {
		s.log.Errorw("failed to transfer eth", "err", err)
		return fmt.Errorf("fail to send tx: %w", err)
	}

	addrInfo.Amount += s.cfg.WithdrawalAmount
	totalInfo.Amount += s.cfg.WithdrawalAmount

	if err = s.db.UpdateAddrInfo(ctx, targetAddr, addrInfo); err != nil {
		return err
	}

	if err = s.db.UpdateTotalInfo(ctx, totalInfo); err != nil {
		return err
	}

	return nil
}

func (s *Service) transferETH(ctx context.Context, toAddress common.Address) error {
	nonce, err := s.client.PendingNonceAt(ctx, s.cfg.Account.Address)
	if err != nil {
		return err
	}

	value := new(big.Int).SetUint64(s.cfg.WithdrawalAmount)

	gasLimit := uint64(21000) // in units
	gasPrice, err := s.client.SuggestGasPrice(ctx)
	if err != nil {
		return err
	}

	var bytes []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, bytes)

	// signer := types.LatestSignerForChainID(s.cfg.ChainID)

	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, s.cfg.Account.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	err = s.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to send tx: %w", err)
	}

	s.log.Infof("tx sent: %s", signedTx.Hash().Hex())
	s.log.Infof("faucetAddress %v funded successfully", toAddress)

	return nil
}
