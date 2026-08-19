package midtrans_test

import (
	"testing"

	"github.com/Sall-lah/store_order/internal/db"
	"github.com/Sall-lah/store_order/internal/integration/midtrans"
)

func TestGenerateAndVerifySignature(t *testing.T) {
	serverKey := "SB-Mid-server-sample-key-12345"
	orderID := "ORD-20260819-TEST01"
	statusCode := "200"
	grossAmount := "350000.00"

	// 1. Generate signature
	sig := midtrans.GenerateSignature(orderID, statusCode, grossAmount, serverKey)
	if sig == "" {
		t.Fatal("expected non-empty SHA-512 signature")
	}

	// 2. Verify with valid signature
	notif := midtrans.WebhookNotification{
		OrderID:           orderID,
		StatusCode:        statusCode,
		GrossAmount:       grossAmount,
		SignatureKey:      sig,
		TransactionStatus: "settlement",
		FraudStatus:       "accept",
	}

	if !midtrans.VerifySignature(serverKey, notif) {
		t.Errorf("expected signature to verify successfully")
	}

	// 3. Reject tampered signature
	tamperedNotif := notif
	tamperedNotif.GrossAmount = "100000.00"
	if midtrans.VerifySignature(serverKey, tamperedNotif) {
		t.Errorf("expected tampered signature to be rejected")
	}

	// 4. Reject wrong server key
	if midtrans.VerifySignature("wrong-key", notif) {
		t.Errorf("expected wrong server key to fail verification")
	}
}

func TestDetermineOrderStatus(t *testing.T) {
	tests := []struct {
		name           string
		notif          midtrans.WebhookNotification
		expectedStatus db.OrderStatus
		expectedOK     bool
	}{
		{
			name: "Settlement payment",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "settlement",
			},
			expectedStatus: db.OrderStatusPaid,
			expectedOK:     true,
		},
		{
			name: "Capture with accept fraud status",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "capture",
				FraudStatus:       "accept",
			},
			expectedStatus: db.OrderStatusPaid,
			expectedOK:     true,
		},
		{
			name: "Capture with challenge fraud status",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "capture",
				FraudStatus:       "challenge",
			},
			expectedStatus: db.OrderStatusPendingPayment,
			expectedOK:     true,
		},
		{
			name: "Pending payment",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "pending",
			},
			expectedStatus: db.OrderStatusPendingPayment,
			expectedOK:     true,
		},
		{
			name: "Cancelled payment",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "cancel",
			},
			expectedStatus: db.OrderStatusCancelled,
			expectedOK:     true,
		},
		{
			name: "Expired payment",
			notif: midtrans.WebhookNotification{
				TransactionStatus: "expire",
			},
			expectedStatus: db.OrderStatusExpired,
			expectedOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := midtrans.DetermineOrderStatus(tt.notif)
			if ok != tt.expectedOK {
				t.Fatalf("expected ok=%v, got %v", tt.expectedOK, ok)
			}
			if status != tt.expectedStatus {
				t.Fatalf("expected status=%v, got %v", tt.expectedStatus, status)
			}
		})
	}
}
