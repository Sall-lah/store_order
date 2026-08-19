package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Sall-lah/store_order/internal/integration/midtrans"
	"github.com/Sall-lah/store_order/internal/middleware"
	"github.com/Sall-lah/store_order/internal/service"
)

// WebhookHandler exposes public callback endpoints for payment gateway notifications.
type WebhookHandler struct {
	orderService service.OrderService
}

// NewWebhookHandler constructs a WebhookHandler instance.
// Why: Provides dedicated transport routing for asynchronous payment gateway notifications.
func NewWebhookHandler(orderService service.OrderService) *WebhookHandler {
	return &WebhookHandler{orderService: orderService}
}

// HandleMidtrans processes asynchronous HTTP POST webhook notifications from Midtrans.
// Why: Receives settlement, fraud, and expiration events and updates order status with cryptographic verification.
func (h *WebhookHandler) HandleMidtrans(w http.ResponseWriter, r *http.Request) {
	middleware.LimitRequestBody(w, r, middleware.DefaultMaxBodyBytes)

	var notif midtrans.WebhookNotification
	if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
		log.Printf("[Midtrans Webhook] Failed to parse JSON body: %v", err)
		RespondError(w, http.StatusBadRequest, "bad_request", "Invalid notification payload format.")
		return
	}

	log.Printf("[Midtrans Webhook] Received notification for Order %s: status=%s, fraud=%s, gross_amount=%s",
		notif.OrderID, notif.TransactionStatus, notif.FraudStatus, notif.GrossAmount)

	if err := h.orderService.ProcessMidtransWebhook(r.Context(), notif); err != nil {
		log.Printf("[Midtrans Webhook] Error processing notification: %v", err)
		RespondError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Notification received and processed successfully.",
	})
}
