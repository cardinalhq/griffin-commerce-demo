package cart

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/cardinalhq/griffin-commerce-demo/common"
)

var (
	httpClient     *http.Client
	catalogBaseURL string
)

// InitProductClient initializes the HTTP client for the product service
func InitProductClient(baseURL string) {
	catalogBaseURL = baseURL
	httpClient = &http.Client{
		Timeout: 5 * time.Second,
	}
}

// GetProduct fetches product details from the catalog service
func GetProduct(productID string) (*common.Product, error) {
	url := fmt.Sprintf("%s/api/products/%s", catalogBaseURL, productID)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch product: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("product not found: %s", productID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch product, status: %d", resp.StatusCode)
	}

	var product common.Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return nil, fmt.Errorf("failed to decode product response: %w", err)
	}

	return &product, nil
}
