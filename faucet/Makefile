all: tidy fmt lint test

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: go-test
go-test:
	go test -v -shuffle=on -count=1 -race -timeout 20m ./...

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

.PHONY: demo-tls
demo-tls:
	rm -rf ./_db_data
	go run ./cmd/main.go --web-allowed-origins "https://localhost" --web-backend-host "https://localhost:443/fund" \
	--ethereum-private-key "b0057716d5917badaf911b193b12b910811c1497b5bada8d7711f758981c3773" --ethereum-api "http://localhost:8545"

.PHONY: node-start
node-start:
	docker run --name ganache-node --rm --detach --publish 8545:8545 trufflesuite/ganache:latest \
 		--accounts 10 --wallet.deterministic

.PHONY: node-stop
node-stop:
	docker stop ganache-node

.PHONY: wait-for-it
wait-for-it:
	./scripts/wait-for-it.sh -h 127.0.0.1 -p 8545
	sleep 3

.PHONY: test
test: node-start wait-for-it go-test node-stop