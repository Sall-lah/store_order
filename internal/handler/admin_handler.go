package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/middleware"
	"github.com/Sall-lah/store_order/internal/repository"
	"github.com/Sall-lah/store_order/internal/service"
)

// AdminHandler exposes order operations restricted to administrator roles.
type AdminHandler struct {
	orderService service.OrderService
}

// NewAdminHandler constructs an AdminHandler with injected order service dependencies.
// Why: Provides administrative endpoints for fulfillment workflows and cross-tenant order visibility.
func NewAdminHandler(orderService service.OrderService) *AdminHandler {
	return &AdminHandler{orderService: orderService}
}

// UpdateStatusRequest payload for administrative status transitions.
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// ListOrders retrieves orders across all users with multi-attribute filtering.
// Why: Powers back-office admin dashboard order grids and search interfaces.
func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	statusStr := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))

	var statusFilter *db.OrderStatus
	if statusStr != "" {
		st := db.OrderStatus(statusStr)
		statusFilter = &st
	}

	filter := repository.OrderFilter{
		UserID: userID,
		Status: statusFilter,
		Search: search,
		Limit:  limit,
		Offset: offset,
	}

	ordersResp, err := h.orderService.AdminListOrders(r.Context(), filter)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve admin orders.")
		return
	}

	RespondJSON(w, http.StatusOK, ordersResp)
}

// UpdateOrderStatus transitions an order to a new fulfillment state.
// Why: Enables administrators to mark orders as PROCESSING, SHIPPED, or COMPLETED.
func (h *AdminHandler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	middleware.LimitRequestBody(w, r, middleware.DefaultMaxBodyBytes)

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "bad_request", "Invalid JSON payload.")
		return
	}

	if strings.TrimSpace(req.Status) == "" {
		RespondValidationError(w, map[string]string{"status": "Target status is required."})
		return
	}

	orderResp, err := h.orderService.AdminUpdateStatus(r.Context(), orderID, req.Status)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "update_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, orderResp)
}
