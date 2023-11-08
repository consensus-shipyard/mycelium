all: tidy fmt lint tests

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: build
build:
	go build -o ./faucet ./cmd/main.go

.PHONY: api-tests
api-tests:
	go test -v -shuffle=on -count=1 -race -timeout 20m ./internal/tests

.PHONY: fmt
fmt:
	gofmt -w -s .
	goimports -w -local "github.com/consensus-shipyard/" .

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: vulncheck
vulncheck:
	govulncheck ./...

.PHONY: cert
cert:
	mkcert -cert-file "./_cert/cert.pem" -key-file "./_cert/key.pem" faucet.com 127.0.0.1 localhost

.PHONY: demo
demo:
	rm -rf ./_db_data
	go run ./cmd/main.go --web-host "127.0.0.1:8000" --web-allowed-origins "http://localhost:8000" --web-backend-host "http://localhost:8000/fund" \
 	--ethereum-private-key "b0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773" --ethereum-api "http://localhost:8545"

.PHONY: demo-file-key
demo-file-key:
	rm -rf ./_db_data
	go run ./cmd/main.go --web-host "127.0.0.1:8000" --web-allowed-origins "http://localhost:8000" --web-backend-host "http://localhost:8000/fund" \
 	--ethereum-private-key-file "~/.faucet.key" --ethereum-api "http://localhost:8545"

.PHONY: demo-tls
demo-tls:
	rm -rf ./_db_data
	go run ./cmd/main.go --web-allowed-origins "https://localhost" --web-backend-host "https://localhost:443/fund" \
	--ethereum-private-key "b0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773" --ethereum-api "http://localhost:8545"

.PHONY: node-start
node-start:
	docker run --name ganache-node --rm --detach --publish 8545:8545 trufflesuite/ganache:latest \
 		--accounts 10 --wallet.deterministic --chain.chainId 1

.PHONY: node-stop
node-stop:
	docker stop ganache-node

.PHONY: wait-for-node
wait-for-node:
	./scripts/wait-for-it.sh -h 127.0.0.1 -p 8545
	sleep 3

.PHONY: wait-for-faucet
wait-for-faucet:
	./scripts/wait-for-it.sh -h 127.0.0.1 -p 8000
	sleep 3

.PHONY: faucet-test-start
faucet-test-start: build
	rm -rf ./_db_data
	./faucet --web-host "127.0.0.1:8000" --web-allowed-origins "http://localhost:8000" --web-backend-host "http://localhost:8000/fund" \
 	--ethereum-private-key "b0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773" --ethereum-api "http://localhost:8545"\
 	--faucet-transfer-amount 132 &

.PHONY: faucet-test-stop
faucet-test-stop:
	pgrep -o -f "ethereum-private-key b0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773" | xargs kill -9

.PHONY: e2e-tests
e2e-tests:
	go test -v -shuffle=on -count=1 -race -timeout 20m ./tests/smoke_test.go

.PHONY: tests
tests: node-start wait-for-node api-tests faucet-test-start wait-for-faucet e2e-tests faucet-test-stop node-stop