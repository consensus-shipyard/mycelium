package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	datastore "github.com/ipfs/go-ds-leveldb"
	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	ldbopts "github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/consensus-shipyard/calibration/faucet/internal/types"

	"github.com/consensus-shipyard/calibration/faucet/internal/data"
	faucetDB "github.com/consensus-shipyard/calibration/faucet/internal/db"
	"github.com/consensus-shipyard/calibration/faucet/internal/faucet"
	handler "github.com/consensus-shipyard/calibration/faucet/internal/http"
)

type FaucetTests struct {
	handler        http.Handler
	store          *datastore.Datastore
	db             *faucetDB.Database
	faucetCfg      *faucet.Config
	client         *ethclient.Client
	transferAmount *big.Int
}

const (
	storePath             = "./_store"
	localEthereumNodeURL  = "http://localhost:8545"
	ganacheDefaultChainID = 1
)

func newClient() (*ethclient.Client, error) {
	return ethclient.Dial(localEthereumNodeURL)
}

func Test_Faucet(t *testing.T) {
	store, err := datastore.NewDatastore(storePath, &datastore.Options{
		Compression: ldbopts.NoCompression,
		NoSync:      false,
		Strict:      ldbopts.StrictAll,
		ReadOnly:    false,
	})
	require.NoError(t, err)

	log := logging.Logger("TEST-FAUCET")

	account, err := data.NewAccount(FaucetPrivateKey)
	require.NoError(t, err)

	require.Equal(t, account.Address.String(), FaucetAccount)

	client, err := newClient()
	require.NoError(t, err)

	chainID, err := client.ChainID(context.Background())
	require.NoError(t, err)

	cfg := faucet.Config{
		TotalTransferLimit:   1000,
		AddressTransferLimit: 50,
		TransferAmount:       10,
		Account:              account,
		ChainID:              chainID,
	}

	srv := handler.FaucetHandler(log, client, store, "0.0.1", &cfg)

	db := faucetDB.NewDatabase(store)

	defer func() {
		err = store.Close()
		require.NoError(t, err)
		err = os.RemoveAll(storePath)
		require.NoError(t, err)
	}()

	tests := FaucetTests{
		handler:        srv,
		store:          store,
		db:             db,
		faucetCfg:      &cfg,
		client:         client,
		transferAmount: faucet.TransferAmount(cfg.TransferAmount),
	}

	t.Run("addrsBaseline", tests.addrsBaseline)
	t.Run("clientAvailable", tests.clientAvailable)
	t.Run("fundEmptyAddress", tests.emptyAddress)
	t.Run("fundAddress201EthAddr", tests.fundAddress201EthAddr)
	t.Run("fundAddress201FilecoinAddr", tests.fundAddress201FilecoinAddr)
	t.Run("fundAddressWithMoreThanAllowed", tests.fundAddressWithMoreThanAllowed)
	t.Run("fundAddressWithMoreThanTotal", tests.fundAddressWithMoreThanTotal)
	t.Run("liveness", tests.liveness)
}

func (ft *FaucetTests) clientAvailable(t *testing.T) {
	id, err := ft.client.ChainID(context.Background())
	require.NoError(t, err)
	require.Equal(t, big.NewInt(ganacheDefaultChainID), id)
}

func (ft *FaucetTests) addrsBaseline(t *testing.T) {
	ethAddr1, err := types.NewEthAddressFromHexString(TestAddr1)
	require.NoError(t, err)

	require.True(t, strings.EqualFold(TestAddr1, ethAddr1.ToHex()))

	filecoinAddr, err := ethAddr1.ToFilecoinAddress()
	require.NoError(t, err)

	ethAddr2, err := types.EthAddressFromFilecoinAddress(filecoinAddr)
	require.NoError(t, err)
	require.Equal(t, ethAddr2, ethAddr1)
	require.True(t, strings.EqualFold(TestAddr1, ethAddr2.ToHex()))
}

func (ft *FaucetTests) liveness(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	w := httptest.NewRecorder()
	ft.handler.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	var resp data.LivenessResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Greater(t, resp.LastBlockNumber, uint64(0))
}

func (ft *FaucetTests) emptyAddress(t *testing.T) {
	req := data.FundRequest{Address: ""}

	body, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/fund", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	ft.handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func (ft *FaucetTests) fundAddress201EthAddr(t *testing.T) {
	ft.fundAddress(t, TestAddr1, TestAddr1)
}

func (ft *FaucetTests) fundAddress201FilecoinAddr(t *testing.T) {
	ft.fundAddress(t, FilecoinTestAddr1, TestAddr1)
}

func (ft *FaucetTests) fundAddress(t *testing.T, fundAddr, checkAddr string) {
	block, err := ft.client.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)

	oldBalance, err := ft.client.BalanceAt(context.Background(), common.HexToAddress(checkAddr), block.Number())
	require.NoError(t, err)

	req := data.FundRequest{Address: fundAddr}

	body, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/fund", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	ft.handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusCreated, w.Code)

	block, err = ft.client.BlockByNumber(context.Background(), nil)
	require.NoError(t, err)

	newBalance, err := ft.client.BalanceAt(context.Background(), common.HexToAddress(checkAddr), block.Number())
	require.NoError(t, err)

	require.Equal(t, new(big.Int).Add(oldBalance, ft.transferAmount), newBalance)
}

// fundAddressWithMoreThanAllowed tests that exceeding daily allowed funds per address is not allowed.
func (ft *FaucetTests) fundAddressWithMoreThanAllowed(t *testing.T) {
	targetAddr := common.HexToAddress(TestAddr1)

	err := ft.db.UpdateAddrInfo(context.Background(), targetAddr, data.AddrInfo{
		Amount:         ft.faucetCfg.AddressTransferLimit,
		LatestTransfer: time.Now(),
	})
	require.NoError(t, err)

	req := data.FundRequest{Address: FilecoinTestAddr1}

	body, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/fund", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	ft.handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	got := w.Body.String()
	exp := faucet.ErrExceedAddrAllowedFunds.Error()
	if !strings.Contains(got, exp) {
		t.Logf("\t\tTest %s:\tGot : %v", t.Name(), got)
		t.Logf("\t\tTest %s:\tExp: %v", t.Name(), exp)
		t.Fatalf("\t\tTest %s:\tShould get the expected result.", t.Name())
	}
}

// fundAddressWithMoreThanAllowed tests that exceeding daily allowed funds per address is not allowed.
func (ft *FaucetTests) fundAddressWithMoreThanTotal(t *testing.T) {
	err := ft.db.UpdateTotalInfo(context.Background(), data.TotalInfo{
		Amount:         ft.faucetCfg.TotalTransferLimit,
		LatestTransfer: time.Now(),
	})
	require.NoError(t, err)

	req := data.FundRequest{Address: FilecoinTestAddr2}

	body, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/fund", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	ft.handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	got := w.Body.String()
	exp := faucet.ErrExceedTotalAllowedFunds.Error()
	if !strings.Contains(got, exp) {
		t.Logf("\t\tTest %s:\tGot : %v", t.Name(), got)
		t.Logf("\t\tTest %s:\tExp: %v", t.Name(), exp)
		t.Fatalf("\t\tTest %s:\tShould get the expected result.", t.Name())
	}
}
