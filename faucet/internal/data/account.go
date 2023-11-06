package data

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type EthereumAccount struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
	Address    common.Address
}

func NewAccount(key string) (*EthereumAccount, error) {
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		return nil, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	addr := crypto.PubkeyToAddress(*publicKeyECDSA)

	return &EthereumAccount{
		PrivateKey: privateKey,
		PublicKey:  publicKeyECDSA,
		Address:    addr,
	}, nil
}
