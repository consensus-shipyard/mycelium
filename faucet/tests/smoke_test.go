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
)

const (
	TestAddr3 = "0xE11BA2b4D45Eaed5996Cd0823791E0C93114882d"
	TestAddr4 = "0xd03ea8624C8C5987235048901fB614fDcA89b117"
)

func Test_Smoke_Service(t *testing.T) {
	ctx := context.Background()

	transferAmount := new(big.Int).SetUint64(132)

	client, err := ethclient.Dial("http://127.0.0.1:8545")
	require.NoError(t, err)

	oldBalance := getBalance(ctx, t, client, TestAddr3)

	sendRequest(t, TestAddr3)
	sendRequest(t, TestAddr4)

	newBalance := getBalance(ctx, t, client, TestAddr3)
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
		fmt.Sprintf(`{"address": "%s"}`, common.HexToAddress(to)),
	)

	r, err := http.NewRequest("POST", "http://localhost:8000/fund", bytes.NewBuffer(body))
	require.NoError(t, err)

	r.Header.Add("Content-Type", "application/json")

	httpClient := &http.Client{}
	res, err := httpClient.Do(r)
	require.NoError(t, err)

	defer res.Body.Close()
}
