package data

import "time"

type FundRequest struct {
	Address string `json:"address"`
}

type AddrInfo struct {
	Amount         uint64    `json:"amount"`
	LatestTransfer time.Time `json:"latest_transfer"`
}

type TotalInfo struct {
	Amount         uint64    `json:"amount"`
	LatestTransfer time.Time `json:"latest_transfer"`
}
