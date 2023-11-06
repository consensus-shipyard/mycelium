package data

type LivenessResponse struct {
	Build           string `json:"build"`
	LastBlockNumber uint64 `json:"n"`
	LastBlockTime   string `json:"time"`
	Host            string `json:"host"`
	ServiceVersion  string `json:"service_version"`
}
