package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/multiformats/go-varint"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/filecoin-project/go-address"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
)

const (
	EthAddressLength    = 20
	EthAddressHexLength = 40
)

var maskedIDPrefix = [20 - 8]byte{0xff}
var ErrInvalidAddress = errors.New("invalid Filecoin Eth address")

type EthAddress [EthAddressLength]byte

func NewEthAddressFromHexString(hexAddr string) (EthAddress, error) {
	a, err := hexutil.Decode(hexAddr)
	if err != nil {
		return EthAddress{}, fmt.Errorf("failed to decode %s", hexAddr)
	}
	var addr [EthAddressLength]byte
	copy(addr[:], a)
	return addr, nil
}

func (ea EthAddress) ToFilecoinAddress() (address.Address, error) {
	if ea.IsMaskedID() {
		// This is a masked ID address.
		id := binary.BigEndian.Uint64(ea[12:])
		return address.NewIDAddress(id)
	}

	// Otherwise, translate the address into an address controlled by the
	// Ethereum Address Manager.
	addr, err := address.NewDelegatedAddress(builtintypes.EthereumAddressManagerActorID, ea[:])
	if err != nil {
		return address.Undef, fmt.Errorf("failed to translate supplied address (%s) into a "+
			"Filecoin f4 address: %w", hex.EncodeToString(ea[:]), err)
	}
	return addr, nil
}

func (ea EthAddress) IsMaskedID() bool {
	return bytes.HasPrefix(ea[:], maskedIDPrefix[:])
}

func (ea EthAddress) ToBytes() []byte {
	return ea[:]
}

func (ea EthAddress) ToHex() string {
	return hexutil.Encode(ea[:])
}

func EthAddressFromFilecoinAddressString(addr string) (common.Address, error) {
	if addr == "" {
		return common.Address{}, fmt.Errorf("empty address string")
	}

	filecoinAddr, err := address.NewFromString(addr)
	if err != nil {
		return common.Address{}, err
	}

	ethAddr, err := EthAddressFromFilecoinAddress(filecoinAddr)
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(ethAddr[:]), nil
}

func EthAddressFromFilecoinAddress(addr address.Address) (EthAddress, error) {
	switch addr.Protocol() {
	case address.ID:
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return EthAddress{}, err
		}
		var ethaddr EthAddress
		ethaddr[0] = 0xff
		binary.BigEndian.PutUint64(ethaddr[12:], id)
		return ethaddr, nil
	case address.Delegated:
		payload := addr.Payload()
		namespace, n, err := varint.FromUvarint(payload)
		if err != nil {
			return EthAddress{}, xerrors.Errorf("invalid delegated address namespace in: %s", addr)
		}
		payload = payload[n:]
		if namespace != builtintypes.EthereumAddressManagerActorID {
			return EthAddress{}, ErrInvalidAddress
		}
		ethAddr, err := CastEthAddress(payload)
		if err != nil {
			return EthAddress{}, err
		}
		if ethAddr.IsMaskedID() {
			return EthAddress{}, xerrors.Errorf("f410f addresses cannot embed masked-ID payloads: %s", ethAddr)
		}
		return ethAddr, nil
	}
	return EthAddress{}, ErrInvalidAddress
}

func CastEthAddress(b []byte) (EthAddress, error) {
	var a EthAddress
	if len(b) != EthAddressLength {
		return EthAddress{}, xerrors.Errorf("cannot parse bytes into an EthAddress: incorrect input length")
	}
	copy(a[:], b[:])
	return a, nil
}
