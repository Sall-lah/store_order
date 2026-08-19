package handler

import (
	"net/http"
	"time"

	"github.com/Sall-lah/store_order/internal/db"
)

// HealthResponse represents the health probe response payload.
type HealthResponse struct {
	Status    string            `json:"status"`
	Service   string            `json:"service"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// HealthHandler manages system health probes and liveness checks.
type HealthHandler struct {
	dbClient *db.PrismaClient
}

// NewHealthHandler constructs a HealthHandler instance.
// Why: Provides container orchestrators and load balancers with deterministic status probes.
func NewHealthHandler(dbClient *db.PrismaClient) *HealthHandler {
	return &HealthHandler{dbClient: dbClient}
}

// Check returns HTTP 200 with service health metadata.
// Why: Allows NGINX gateway and container healthchecks to verify that the service is operational.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	checks["database"] = "UP"
	if h.dbClient == nil {
		checks["database"] = "STANDBY"
	}

	RespondJSON(w, http.StatusOK, HealthResponse{
		Status:    "UP",
		Service:   "store_order",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}
