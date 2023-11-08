package tests

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"

	"github.com/consensus-shipyard/calibration/faucet/internal/faucet"

	"github.com/consensus-shipyard/calibration/faucet/internal/tests"
)

func Test_Smoke_Service(t *testing.T) {
	ctx := context.Background()

	// this amount must be equal to amount in faucet-test-start in the Makefile
	transferAmount := faucet.TransferAmount(13)

	client, err := ethclient.Dial("http://127.0.0.1:8545")
	require.NoError(t, err)

	oldBalance := getBalance(ctx, t, client, tests.TestAddr3)

	sendRequest(t, tests.FilecoinTestAddr3)
	sendRequest(t, tests.FilecoinTestAddr4)

	newBalance := getBalance(ctx, t, client, tests.TestAddr3)
	require.Equal(t, new(big.Int).Add(oldBalance, transferAmount), newBalance)
}

func getBalance(ctx context.Context, t *testing.T, client *ethclient.Client, account string) *big.Int {
	block, err := client.BlockByNumber(ctx, nil)
	require.NoError(t, err)

	b, err := client.BalanceAt(ctx, common.HexToAddress(account), block.Number())
	require.NoError(t, err)

	return b
}

func sendRequest(t *testing.T, to string) {
	body := []byte(
		fmt.Sprintf(`{"address": "%s"}`, to),
	)

	r, err := http.NewRequest("POST", "http://localhost:8000/fund", bytes.NewBuffer(body))
	require.NoError(t, err)

	r.Header.Add("Content-Type", "application/json")

	httpClient := &http.Client{}
	res, err := httpClient.Do(r)
	require.NoError(t, err)

	defer res.Body.Close()
}
