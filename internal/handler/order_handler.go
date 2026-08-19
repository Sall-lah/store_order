package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Sall-lah/store_order/internal/middleware"
	"github.com/Sall-lah/store_order/internal/service"
)

// OrderHandler exposes customer-facing HTTP endpoints for checkout, tracking, and cancellation.
type OrderHandler struct {
	orderService service.OrderService
}

// NewOrderHandler constructs an OrderHandler instance.
// Why: Provides dependency injection of order service logic for customer HTTP transport routes.
func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

// Checkout processes customer order placement and Snap token issuance.
// Why: Entry point for customer checkout operations after shopping cart finalization.
func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		RespondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
		return
	}

	middleware.LimitRequestBody(w, r, middleware.DefaultMaxBodyBytes)

	var req service.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "bad_request", "Invalid JSON payload.")
		return
	}

	if len(req.Items) == 0 {
		RespondValidationError(w, map[string]string{"items": "At least one item is required for checkout."})
		return
	}

	orderResp, err := h.orderService.Checkout(r.Context(), user.ID, user.Email, req)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "checkout_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusCreated, orderResp)
}

// GetOrder retrieves a specific customer order.
// Why: Allows customers to view full order status, line items, and payment instructions.
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		RespondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
		return
	}

	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	orderResp, err := h.orderService.GetCustomerOrder(r.Context(), user.ID, orderID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
			RespondError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		RespondError(w, http.StatusNotFound, "not_found", "Order not found.")
		return
	}

	RespondJSON(w, http.StatusOK, orderResp)
}

// ListOrders retrieves paginated order history for the authenticated customer.
// Why: Powers the user dashboard purchase history screen.
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		RespondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	ordersResp, err := h.orderService.ListCustomerOrders(r.Context(), user.ID, limit, offset)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve order history.")
		return
	}

	RespondJSON(w, http.StatusOK, ordersResp)
}

// CancelOrder permits a customer to cancel an unpaid order.
// Why: Allows customers to abort orders while in PENDING_PAYMENT status and trigger stock release.
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUserFromContext(r.Context())
	if !ok || strings.TrimSpace(user.ID) == "" {
		RespondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required.")
		return
	}

	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		RespondError(w, http.StatusBadRequest, "bad_request", "Order ID parameter is required.")
		return
	}

	orderResp, err := h.orderService.CancelCustomerOrder(r.Context(), user.ID, orderID)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "cancellation_failed", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, orderResp)
}
