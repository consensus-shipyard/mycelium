// This package implements a simple test tool for debugging.
package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	fmt.Println("--------")
	ctx := context.Background()
	client, err := ethclient.Dial("http://127.0.0.1:8545")
	if err != nil {
		log.Fatal("client:", err)
	}

	privateKey, err := crypto.HexToECDSA("50751ea8c272c320c197ba6fd94d5ee8ba065d614790e9bfeca4ba1a162f72e9")
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	from := crypto.PubkeyToAddress(*publicKeyECDSA)

	fmt.Println("Account address:", from)

	balance, err := client.BalanceAt(context.Background(), from, nil)
	if err != nil {
		log.Fatal("BalanceAt:", err)
	}

	fmt.Println("Account balance:", balance)

	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		log.Fatal("PendingNonceAt:", err)
	}
	fmt.Println("Nonce: ", nonce)

	value := big.NewInt(1) // in wei (1 eth)

	to := common.HexToAddress("0xFFcf8FDEE72ac11b5c542428B35EEF5769C409f0")

	gasTipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatal("SuggestGasTipCap:", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		log.Fatal("ChainID:", err)
	}
	fmt.Println("ChainID: ", chainID)

	block, err := client.BlockByNumber(ctx, nil)
	if err != nil {
		log.Fatal("BlockByNumber: ", err)
	}
	baseFee := block.BaseFee()
	gasFeeCap := new(big.Int).SetUint64(1500000000)
	gasFeeCap.Add(gasFeeCap, baseFee.Mul(baseFee, big.NewInt(2)))

	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:      from,
		To:        &to,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Value:     value,
		Data:      nil,
	})
	if err != nil {
		log.Fatal("EstimateGas: ", err)
	}

	gasLimit += gasLimit / 5

	rawTx := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
		Data:      nil,
	}

	fmt.Println("GasFeeCap: ", gasFeeCap)
	fmt.Println("GasTipCap: ", gasTipCap)
	fmt.Println("GasLimit: ", gasLimit)

	signer := types.LatestSignerForChainID(chainID)

	signedTx, err := types.SignNewTx(privateKey, signer, rawTx)
	if err != nil {
		log.Fatal("SignNewTx:", err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: %s", signedTx.Hash().Hex())
}
