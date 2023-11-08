package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/consensus-shipyard/calibration/faucet/internal/tests"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/builtin"
)

func TestParseEthAddr(t *testing.T) {
	testcases := []uint64{
		1, 2, 3, 100, 101,
	}
	for _, id := range testcases {
		addr, err := address.NewIDAddress(id)
		require.Nil(t, err)

		eaddr, err := EthAddressFromFilecoinAddress(addr)
		require.Nil(t, err)

		faddr, err := eaddr.ToFilecoinAddress()
		require.Nil(t, err)

		require.Equal(t, addr, faddr)
	}

	a, err := NewEthAddressFromHexString("0xFFcf8FDEE72ac11b5c542428B35EEF5769C409f0")
	require.NoError(t, err)
	fmt.Println(a.ToFilecoinAddress())
}

func TestSetup(t *testing.T) {
	a, err := NewEthAddressFromHexString(tests.TestAddr1)
	require.NoError(t, err)

	f, err := a.ToFilecoinAddress()
	require.NoError(t, err)

	fmt.Println(f)

	a, err = NewEthAddressFromHexString(tests.TestAddr2)
	require.NoError(t, err)

	f, err = a.ToFilecoinAddress()
	require.NoError(t, err)

	fmt.Println(f)

	a, err = NewEthAddressFromHexString(tests.TestAddr3)
	require.NoError(t, err)

	f, err = a.ToFilecoinAddress()
	require.NoError(t, err)

	fmt.Println(f)

	a, err = NewEthAddressFromHexString(tests.TestAddr4)
	require.NoError(t, err)

	f, err = a.ToFilecoinAddress()
	require.NoError(t, err)

	fmt.Println(f)
}

func TestMaskedIDInF4(t *testing.T) {
	addr, err := address.NewIDAddress(100)
	require.NoError(t, err)

	eaddr, err := EthAddressFromFilecoinAddress(addr)
	require.NoError(t, err)

	badaddr, err := address.NewDelegatedAddress(builtin.EthereumAddressManagerActorID, eaddr[:])
	require.NoError(t, err)

	_, err = EthAddressFromFilecoinAddress(badaddr)
	require.Error(t, err)
}
