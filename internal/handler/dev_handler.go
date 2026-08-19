package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Sall-lah/store_order/internal/service"
)

// DevHandler provides developer testing endpoints for offline simulation of payment states.
type DevHandler struct {
	orderService service.OrderService
}

// NewDevHandler constructs a DevHandler instance.
// Why: Provides developer endpoints for triggering order state transitions without external webhook tunnels.
func NewDevHandler(orderService service.OrderService) *DevHandler {
	return &DevHandler{orderService: orderService}
}

// SimulateSuccess transitions an order to PAID and inserts an order.paid outbox event.
// Why: Enables instant offline developer verification of post-payment stock deduction and fulfillment workflows.
func (h *DevHandler) SimulateSuccess(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	orderResp, err := h.orderService.SimulatePaymentSuccess(r.Context(), orderID)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "simulation_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Order payment simulated successfully. Order transitioned to PAID and order.paid event queued.",
		"order":   orderResp,
	})
}

// SimulateCancel transitions an order to CANCELLED and queues an order.cancelled outbox event.
// Why: Enables rapid testing of stock release and cancellation pipelines.
func (h *DevHandler) SimulateCancel(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	orderResp, err := h.orderService.SimulateOrderCancel(r.Context(), orderID)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "simulation_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Order cancellation simulated successfully. Order transitioned to CANCELLED and order.cancelled event queued.",
		"order":   orderResp,
	})
}

// SimulateExpire transitions an order to EXPIRED and queues an order.expired outbox event.
// Why: Enables testing of TTL payment window expiry and cleanup workers.
func (h *DevHandler) SimulateExpire(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	orderResp, err := h.orderService.SimulateOrderExpire(r.Context(), orderID)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "simulation_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Order expiration simulated successfully. Order transitioned to EXPIRED and order.expired event queued.",
		"order":   orderResp,
	})
}
