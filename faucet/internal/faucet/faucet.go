package faucet

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
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

	gasTipCap, err := s.client.SuggestGasTipCap(ctx)
	if err != nil {
		return fmt.Errorf("failed to suggest gas tip: %w", err)
	}

	// https://github.com/ethereum/go-ethereum/issues/23125
	block, err := s.client.BlockByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get block: %w", err)
	}
	baseFee := block.BaseFee()
	gasFeeCap := new(big.Int).SetUint64(1500000000)
	gasFeeCap.Add(gasFeeCap, baseFee.Mul(baseFee, big.NewInt(2)))

	gasLimit, err := s.client.EstimateGas(ctx, ethereum.CallMsg{
		From:      s.cfg.Account.Address,
		To:        &to,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Data:      nil,
	})
	if err != nil {
		s.log.Errorw(
			"failed to estimate gas price",
			"to", to.String(),
			"from", s.cfg.Account.Address,
			"GasFeeCap", gasFeeCap,
			"gasTipCap", gasTipCap,
			"baseFee", baseFee,
		)
		return fmt.Errorf("failed to estimate gas price: %w", err)
	}

	gasLimit += gasLimit / 5

	rawTx := &types.DynamicFeeTx{
		ChainID:   s.cfg.ChainID,
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
		Data:      nil,
	}

	signer := types.LatestSignerForChainID(s.cfg.ChainID)

	signedTx, err := types.SignNewTx(s.cfg.Account.PrivateKey, signer, rawTx)
	if err != nil {
		return err
	}

	err = s.client.SendTransaction(ctx, signedTx)
	if err != nil {
		s.log.Errorw(
			"failed to send tx", "hash", signedTx.Hash(),
			"GasFeeCap", gasFeeCap,
			"gas", gasLimit,
			"gasTipCap", gasTipCap,
			"baseFee", baseFee,
		)
		return fmt.Errorf("failed to send tx: %w", err)
	}

	s.log.Infof("tx sent: %s", signedTx.Hash().Hex())
	s.log.Infof("faucetAddress %v funded successfully", to)

	return nil
}

func TransferAmount(amount uint64) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(amount), big.NewInt(params.Ether))
}
