package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/griffincommerce/demo/common"
)

var (
	productCache      []common.Product
	productCacheMutex sync.RWMutex
	catalogClient     *http.Client
)

func main() {
	shutdown, err := common.InitTelemetry("recommendations-service")
	if err != nil {
		log.Printf("Failed to initialize telemetry: %v", err)
	}
	defer shutdown()

	catalogClient = &http.Client{Timeout: 10 * time.Second}

	if err := loadProductsFromCatalog(); err != nil {
		log.Printf("Warning: Failed to preload products: %v", err)
	}

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := loadProductsFromCatalog(); err != nil {
				log.Printf("Failed to refresh product cache: %v", err)
			}
		}
	}()

	r := mux.NewRouter()

	r.Use(common.LoggingMiddleware)
	r.Use(common.CorrelationIDMiddleware)
	r.Use(common.TracingMiddleware("recommendations-service"))
	r.Use(common.CORSMiddleware)

	r.HandleFunc("/health", HealthHandler).Methods("GET")
	r.HandleFunc("/api/recommendations", GetRecommendationsHandler).Methods("GET")
	r.HandleFunc("/api/recommendations/product/{id}", GetProductRecommendationsHandler).Methods("GET")

	port := getPort()
	log.Printf("Recommendations Service starting on port %d", port)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getPort() int {
	if port := os.Getenv("PORT"); port != "" {
		var p int
		if _, err := fmt.Sscanf(port, "%d", &p); err != nil {
			log.Printf("Failed to parse port: %v", err)
			return 8085
		}
		return p
	}
	return 8085
}

func loadProductsFromCatalog() error {
	catalogURL := os.Getenv("CATALOG_SERVICE_URL")
	if catalogURL == "" {
		catalogURL = "http://localhost:8080"
	}

	resp, err := catalogClient.Get(catalogURL + "/api/products")
	if err != nil {
		return fmt.Errorf("failed to fetch products: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("catalog service returned status %d", resp.StatusCode)
	}

	var products []common.Product

	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return fmt.Errorf("failed to decode products: %w", err)
	}

	productCacheMutex.Lock()
	productCache = products
	productCacheMutex.Unlock()

	log.Printf("Loaded %d products from catalog", len(products))
	return nil
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := common.HealthResponse{
		Status:    "healthy",
		Service:   "recommendations-service",
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}
	if err := common.WriteJSONResponse(w, health, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

type RecommendationsResponse struct {
	Products []common.Product `json:"products"`
	Type     string           `json:"type"`
}

func GetRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	count := 5
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil && c > 0 && c <= 20 {
			count = c
		}
	}

	recommendations := getRandomRecommendations(count, "")

	response := RecommendationsResponse{
		Products: recommendations,
		Type:     "trending",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

func GetProductRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productID := vars["id"]

	count := 4
	if countStr := r.URL.Query().Get("count"); countStr != "" {
		if c, err := strconv.Atoi(countStr); err == nil && c > 0 && c <= 20 {
			count = c
		}
	}

	recommendations := getRandomRecommendations(count, productID)

	response := RecommendationsResponse{
		Products: recommendations,
		Type:     "related",
	}

	if err := common.WriteJSONResponse(w, response, http.StatusOK); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

func getRandomRecommendations(count int, excludeID string) []common.Product {
	productCacheMutex.RLock()
	defer productCacheMutex.RUnlock()

	if len(productCache) == 0 {
		return []common.Product{}
	}

	var eligibleProducts []common.Product
	for _, product := range productCache {
		if product.ID != excludeID && product.Stock > 0 {
			eligibleProducts = append(eligibleProducts, product)
		}
	}

	if len(eligibleProducts) == 0 {
		return []common.Product{}
	}

	rand.Shuffle(len(eligibleProducts), func(i, j int) {
		eligibleProducts[i], eligibleProducts[j] = eligibleProducts[j], eligibleProducts[i]
	})

	if count > len(eligibleProducts) {
		count = len(eligibleProducts)
	}

	return eligibleProducts[:count]
}
