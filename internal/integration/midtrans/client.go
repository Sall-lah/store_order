package midtrans

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TransactionDetails contains order identifier and gross checkout total.
type TransactionDetails struct {
	OrderID     string  `json:"order_id"`
	GrossAmount float64 `json:"gross_amount"`
}

// CustomerDetails contains basic customer identity metadata.
type CustomerDetails struct {
	FirstName string `json:"first_name"`
	Email     string `json:"email"`
}

// ItemDetail represents a single line item passed to the Midtrans Snap modal.
type ItemDetail struct {
	ID       string  `json:"id"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Name     string  `json:"name"`
}

// SnapTransactionRequest defines the payload structure expected by Midtrans Snap API.
type SnapTransactionRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	CustomerDetails    CustomerDetails    `json:"customer_details"`
	ItemDetails        []ItemDetail       `json:"item_details,omitempty"`
}

// SnapResponse contains the transaction token and redirect URL issued by Midtrans.
type SnapResponse struct {
	Token       string `json:"token"`
	RedirectURL string `json:"redirect_url"`
}

// Client defines the interface for creating Midtrans Snap transactions.
type Client interface {
	CreateSnapTransaction(ctx context.Context, req SnapTransactionRequest) (*SnapResponse, error)
}

// SnapClient implements Client for Midtrans Snap API.
type SnapClient struct {
	serverKey    string
	isProduction bool
	isDevMode    bool
	httpClient   *http.Client
}

// NewSnapClient constructs a configured SnapClient instance.
// Why: Provides unified payment client configuration with automatic sandbox/production switching and local dev fallback.
func NewSnapClient(serverKey string, isProduction, isDevMode bool) *SnapClient {
	return &SnapClient{
		serverKey:    serverKey,
		isProduction: isProduction,
		isDevMode:    isDevMode,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateSnapTransaction generates a Snap token and payment redirect URL via Midtrans API.
// Why: Enables seamless in-app and hosted payment redirects for customers to complete checkout.
func (c *SnapClient) CreateSnapTransaction(ctx context.Context, req SnapTransactionRequest) (*SnapResponse, error) {
	// If in dev mode and server key is absent or placeholder, produce a deterministic mock token
	if c.isDevMode && (strings.TrimSpace(c.serverKey) == "" || strings.HasPrefix(c.serverKey, "SB-Mid-server-sample")) {
		mockToken := fmt.Sprintf("mock_snap_token_%s", req.TransactionDetails.OrderID)
		mockURL := fmt.Sprintf("https://app.sandbox.midtrans.com/snap/v2/vtweb/%s", mockToken)
		return &SnapResponse{
			Token:       mockToken,
			RedirectURL: mockURL,
		}, nil
	}

	if strings.TrimSpace(c.serverKey) == "" {
		return nil, errors.New("midtrans server key is required but unconfigured")
	}

	endpoint := "https://app.sandbox.midtrans.com/snap/v1/transactions"
	if c.isProduction {
		endpoint = "https://app.midtrans.com/snap/v1/transactions"
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal snap request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create snap http request: %w", err)
	}

	// Midtrans expects Basic Auth with ServerKey as username and empty password
	auth := base64.StdEncoding.EncodeToString([]byte(c.serverKey + ":"))
	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute snap transaction request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errResp map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("midtrans snap error (status %d): %v", resp.StatusCode, errResp)
	}

	var snapResp SnapResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapResp); err != nil {
		return nil, fmt.Errorf("failed to decode snap response: %w", err)
	}

	return &snapResp, nil
}
