package midtrans

import (
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Sall-lah/store_order/internal/db"
)

// WebhookNotification represents the JSON payload dispatched by Midtrans on payment lifecycle events.
type WebhookNotification struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	PaymentType       string `json:"payment_type"`
	TransactionTime   string `json:"transaction_time"`
	TransactionID     string `json:"transaction_id"`
}

// GenerateSignature computes the SHA-512 signature for a Midtrans transaction.
// Why: Implements the exact cryptographic algorithm mandated by Midtrans for webhook verification.
func GenerateSignature(orderID, statusCode, grossAmount, serverKey string) string {
	raw := fmt.Sprintf("%s%s%s%s", orderID, statusCode, grossAmount, serverKey)
	hash := sha512.Sum512([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// VerifySignature validates that an incoming webhook notification was signed by Midtrans using the server key.
// Why: Prevents payment spoofing attacks by ensuring cryptographic authenticity before modifying order status.
func VerifySignature(serverKey string, notif WebhookNotification) bool {
	if strings.TrimSpace(serverKey) == "" || strings.TrimSpace(notif.SignatureKey) == "" {
		return false
	}

	expectedSig := GenerateSignature(notif.OrderID, notif.StatusCode, notif.GrossAmount, serverKey)
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(expectedSig)), []byte(strings.ToLower(notif.SignatureKey))) == 1
}

// DetermineOrderStatus maps Midtrans transaction and fraud statuses to the internal OrderStatus enum.
// Why: Standardizes external payment gateway lifecycle events into consistent internal domain state transitions.
func DetermineOrderStatus(notif WebhookNotification) (db.OrderStatus, bool) {
	status := strings.ToLower(strings.TrimSpace(notif.TransactionStatus))
	fraud := strings.ToLower(strings.TrimSpace(notif.FraudStatus))

	switch status {
	case "capture":
		if fraud == "challenge" {
			return db.OrderStatusPendingPayment, true
		}
		if fraud == "accept" || fraud == "" {
			return db.OrderStatusPaid, true
		}
		return db.OrderStatusPendingPayment, false
	case "settlement":
		return db.OrderStatusPaid, true
	case "pending":
		return db.OrderStatusPendingPayment, true
	case "deny", "cancel":
		return db.OrderStatusCancelled, true
	case "expire":
		return db.OrderStatusExpired, true
	default:
		return db.OrderStatusPendingPayment, false
	}
}
