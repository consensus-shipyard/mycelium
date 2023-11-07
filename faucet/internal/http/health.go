package http

import (
	"context"
	"net/http"
	"os"
	"time"

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
		h.log.Infow("liveness failure", "status", "eth client not ready")
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}

	h.log.Infow("liveness check", "statusCode", statusCode, "method", r.Method, "path", r.URL.Path, "remoteaddr", r.RemoteAddr)

	resp := data.LivenessResponse{
		Host:            host,
		Build:           h.build,
		LastBlockTime:   block.ReceivedAt.String(),
		LastBlockNumber: block.NumberU64(),
		ServiceVersion:  version.Version(),
	}

	if err := web.Respond(r.Context(), w, resp, statusCode); err != nil {
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}
}

// Readiness checks if the components are ready and if not will return a 500 status.
func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := "ok"
	statusCode := http.StatusOK

	h.log.Infow("readiness check", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)

	if _, err := h.client.BlockByNumber(ctx, nil); err != nil {
		status = "eth client not ready"
		statusCode = http.StatusInternalServerError
		h.log.Infow("readiness failure", "status", status)
		web.RespondError(w, statusCode, err)
		return
	}

	resp := struct {
		Status string `json:"status"`
	}{
		Status: status,
	}

	if err := web.Respond(ctx, w, resp, statusCode); err != nil {
		web.RespondError(w, http.StatusInternalServerError, err)
		return
	}
}
