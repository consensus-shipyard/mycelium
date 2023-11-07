package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	datastore "github.com/ipfs/go-ds-leveldb"
	"github.com/stretchr/testify/require"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/consensus-shipyard/calibration/faucet/internal/data"
)

const (
	dbTestStorePath = "./_db_test_store"
	dbTestAddr1     = "0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d"
)

func Test_Faucet(t *testing.T) {
	store, err := datastore.NewDatastore(dbTestStorePath, &datastore.Options{
		Compression: ldbopts.NoCompression,
		NoSync:      false,
		Strict:      ldbopts.StrictAll,
		ReadOnly:    false,
	})
	require.NoError(t, err)

	defer func() {
		err = store.Close()
		require.NoError(t, err)
		err = os.RemoveAll(dbTestStorePath)
		require.NoError(t, err)
	}()

	db := NewDatabase(store)

	ctx := context.Background()

	addr := common.HexToAddress(dbTestAddr1)

	addrInfo, err := db.GetAddrInfo(ctx, addr)
	require.NoError(t, err)
	require.Equal(t, data.AddrInfo{}, addrInfo)

	totalInfo, err := db.GetTotalInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, data.TotalInfo{}, totalInfo)

	newAddrInfo := data.AddrInfo{
		Amount:           12,
		LatestWithdrawal: time.Now(),
	}
	err = db.UpdateAddrInfo(ctx, addr, newAddrInfo)
	require.NoError(t, err)

	addrInfo, err = db.GetAddrInfo(ctx, addr)
	require.NoError(t, err)
	require.Equal(t, newAddrInfo.Amount, addrInfo.Amount)
	require.Equal(t, true, newAddrInfo.LatestWithdrawal.Equal(addrInfo.LatestWithdrawal))

	newTotalInfo := data.TotalInfo{
		Amount:           3000,
		LatestWithdrawal: time.Now(),
	}
	err = db.UpdateTotalInfo(ctx, newTotalInfo)
	require.NoError(t, err)

	totalInfo, err = db.GetTotalInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, newTotalInfo.Amount, totalInfo.Amount)
	require.Equal(t, true, newTotalInfo.LatestWithdrawal.Equal(totalInfo.LatestWithdrawal))
}
