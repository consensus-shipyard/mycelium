package faucet

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
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

type Config struct {
	AllowedOrigins       []string
	TotalTransferLimit   uint64
	AddressTransferLimit uint64
	TransferAmount       uint64
	BackendAddress       string
	Account              *data.EthereumAccount
	ChainID              *big.Int
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
	s.log.Infof("funding address info: %v", addrInfo)

	totalInfo, err := s.db.GetTotalInfo(ctx)
	if err != nil {
		return err
	}
	s.log.Infof("total info: %v", totalInfo)

	if addrInfo.LatestTransfer.IsZero() || time.Since(addrInfo.LatestTransfer) >= 24*time.Hour {
		addrInfo.Amount = 0
		addrInfo.LatestTransfer = time.Now()
	}

	if totalInfo.LatestTransfer.IsZero() || time.Since(totalInfo.LatestTransfer) >= 24*time.Hour {
		totalInfo.Amount = 0
		totalInfo.LatestTransfer = time.Now()
	}

	if totalInfo.Amount >= s.cfg.TotalTransferLimit {
		return ErrExceedTotalAllowedFunds
	}

	if addrInfo.Amount >= s.cfg.AddressTransferLimit {
		return ErrExceedAddrAllowedFunds
	}

	s.log.Infof("funding %v is allowed", targetAddr)

	if err = s.transferETH(ctx, targetAddr); err != nil {
		s.log.Errorw("failed to transfer eth", "err", err)
		return fmt.Errorf("fail to send tx: %w", err)
	}

	addrInfo.Amount += s.cfg.TransferAmount
	totalInfo.Amount += s.cfg.TransferAmount

	if err = s.db.UpdateAddrInfo(ctx, targetAddr, addrInfo); err != nil {
		return err
	}

	if err = s.db.UpdateTotalInfo(ctx, totalInfo); err != nil {
		return err
	}

	return nil
}

func (s *Service) transferETH(ctx context.Context, to common.Address) error {
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*5000*4)
	defer cancel()

	nonce, err := s.client.PendingNonceAt(ctx, s.cfg.Account.Address)
	if err != nil {
		return fmt.Errorf("failed to retrieve nonce: %w", err)
	}

	value := TransferAmount(s.cfg.TransferAmount)

	gasLimit := uint64(21000)            // in units
	gasTipCap := big.NewInt(2000000000)  // maxPriorityFeePerGas = 2 Gwei
	gasFeeCap := big.NewInt(20000000000) // maxFeePerGas = 20 Gwei

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   s.cfg.ChainID,
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
		Data:      nil,
	})

	signer := types.LatestSignerForChainID(s.cfg.ChainID)

	signedTx, err := types.SignTx(tx, signer, s.cfg.Account.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}

	err = s.client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to send tx: %w", err)
	}

	s.log.Infof("tx sent: %s", signedTx.Hash().Hex())
	s.log.Infof("faucetAddress %v funded successfully", to)

	return nil
}

func TransferAmount(amount uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(amount), big.NewInt(params.Ether))
}
