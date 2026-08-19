package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// VariantDTO represents variant data returned by store_product.
type VariantDTO struct {
	ID        string   `json:"id"`
	ProductID string   `json:"product_id"`
	SKU       string   `json:"sku"`
	Size      *string  `json:"size"`
	Color     *string  `json:"color"`
	Price     *float64 `json:"price"`
	Stock     int      `json:"stock"`
	IsActive  bool     `json:"is_active"`
}

// ProductDTO represents product details returned by store_product.
type ProductDTO struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Slug        string       `json:"slug"`
	Description *string      `json:"description"`
	BasePrice   float64      `json:"base_price"`
	Category    string       `json:"category"`
	IsActive    bool         `json:"is_active"`
	Variants    []VariantDTO `json:"variants"`
}

// ItemOrderRequest represents an item requested by a customer at checkout.
type ItemOrderRequest struct {
	ProductID string `json:"productId"`
	VariantID string `json:"variantId"`
	Quantity  int    `json:"quantity"`
}

// ValidatedItem contains authoritative pricing, snapshot metadata, and verified subtotals.
type ValidatedItem struct {
	ProductID   string  `json:"productId"`
	VariantID   string  `json:"variantId"`
	ProductName string  `json:"productName"`
	VariantName string  `json:"variantName"`
	SKU         string  `json:"sku"`
	UnitPrice   float64 `json:"unitPrice"`
	Quantity    int     `json:"quantity"`
	Subtotal    float64 `json:"subtotal"`
}

// Client defines the interface for communicating with store_product.
type Client interface {
	GetProductByID(ctx context.Context, productID string) (*ProductDTO, error)
	ValidateItems(ctx context.Context, requests []ItemOrderRequest) ([]ValidatedItem, float64, error)
}

// HTTPClient implements Client via standard HTTP requests against store_product.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a configured HTTPClient with default connection timeouts.
// Why: Encapsulates service-to-service communication with resilient connection pooling and bounded timeouts.
func NewClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// GetProductByID retrieves product details and variants from store_product.
// Why: Allows the order service to fetch authoritative product metadata and active variant price points.
func (c *HTTPClient) GetProductByID(ctx context.Context, productID string) (*ProductDTO, error) {
	if c.baseURL == "" {
		return nil, errors.New("product service URL is not configured")
	}

	url := fmt.Sprintf("%s/api/v1/products/%s", c.baseURL, productID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create product request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach product service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("product %s not found", productID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("product service returned status %d", resp.StatusCode)
	}

	var product ProductDTO
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode product response: %w", err)
	}

	return &product, nil
}

// ValidateItems checks existence, availability, and calculates server-verified prices for all checkout items.
// Why: Enforces price integrity and prevents client-side price tampering attacks during checkout.
func (c *HTTPClient) ValidateItems(ctx context.Context, requests []ItemOrderRequest) ([]ValidatedItem, float64, error) {
	if len(requests) == 0 {
		return nil, 0, errors.New("order must contain at least one item")
	}

	// Cache products fetched during this checkout to avoid duplicate network calls for multiple variants of the same product
	productCache := make(map[string]*ProductDTO)
	var validatedItems []ValidatedItem
	var totalAmount float64

	for _, itemReq := range requests {
		if itemReq.Quantity <= 0 {
			return nil, 0, fmt.Errorf("invalid quantity %d for product %s", itemReq.Quantity, itemReq.ProductID)
		}

		prod, exists := productCache[itemReq.ProductID]
		if !exists {
			var err error
			prod, err = c.GetProductByID(ctx, itemReq.ProductID)
			if err != nil {
				return nil, 0, err
			}
			if !prod.IsActive {
				return nil, 0, fmt.Errorf("product %q is currently inactive", prod.Name)
			}
			productCache[itemReq.ProductID] = prod
		}

		var targetVariant *VariantDTO
		for _, v := range prod.Variants {
			if v.ID == itemReq.VariantID {
				targetVariant = &v
				break
			}
		}

		if targetVariant == nil {
			return nil, 0, fmt.Errorf("variant %s not found on product %s", itemReq.VariantID, prod.Name)
		}
		if !targetVariant.IsActive {
			return nil, 0, fmt.Errorf("variant %s (%s) is not active", targetVariant.SKU, prod.Name)
		}

		unitPrice := prod.BasePrice
		if targetVariant.Price != nil && *targetVariant.Price > 0 {
			unitPrice = *targetVariant.Price
		}

		subtotal := unitPrice * float64(itemReq.Quantity)
		totalAmount += subtotal

		variantName := targetVariant.SKU
		if targetVariant.Size != nil || targetVariant.Color != nil {
			sizeStr := ""
			if targetVariant.Size != nil {
				sizeStr = *targetVariant.Size
			}
			colorStr := ""
			if targetVariant.Color != nil {
				colorStr = *targetVariant.Color
			}
			variantName = fmt.Sprintf("%s / %s", sizeStr, colorStr)
		}

		validatedItems = append(validatedItems, ValidatedItem{
			ProductID:   prod.ID,
			VariantID:   targetVariant.ID,
			ProductName: prod.Name,
			VariantName: variantName,
			SKU:         targetVariant.SKU,
			UnitPrice:   unitPrice,
			Quantity:    itemReq.Quantity,
			Subtotal:    subtotal,
		})
	}

	return validatedItems, totalAmount, nil
}
