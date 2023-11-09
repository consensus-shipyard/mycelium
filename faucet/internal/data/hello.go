package data

type LivenessResponse struct {
	Build           string `json:"service_build"`
	LastBlockNumber uint64 `json:"last_block_number"`
	LastBlockTime   string `json:"last_block_time"`
	Host            string `json:"service_host"`
	ServiceVersion  string `json:"service_version"`
}
