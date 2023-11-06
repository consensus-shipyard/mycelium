package http

import (
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	logging "github.com/ipfs/go-log/v2"

	"github.com/consensus-shipyard/calibration/faucet/internal/data"
	"github.com/consensus-shipyard/calibration/faucet/internal/platform/web"
	"github.com/consensus-shipyard/calibration/faucet/pkg/version"
)

type Health struct {
	log    *logging.ZapEventLogger
	client *ethclient.Client
	build  string
}

func NewHealth(log *logging.ZapEventLogger, client *ethclient.Client, build string) *Health {
	h := Health{
		log:    log,
		client: client,
		build:  build,
	}

	return &h
}

// Liveness returns status info if the service is alive.
func (h *Health) Liveness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	statusCode := http.StatusOK

	block, err := h.client.BlockByNumber(ctx, nil)
	if err != nil {
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}

	h.log.Infow("liveness", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)

	resp := data.LivenessResponse{
		Host:            host,
		Build:           h.build,
		LastBlockTime:   block.ReceivedAt.String(),
		LastBlockNumber: block.NumberU64(),
		ServiceVersion:  version.Version(),
	}

	if err := web.Respond(r.Context(), w, resp, http.StatusOK); err != nil {
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}
}

// Readiness checks if the components are ready and if not will return a 500 status.
func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.log.Infow("readiness", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)

	resp := struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	}

	if err := web.Respond(ctx, w, resp, http.StatusOK); err != nil {
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}
}
