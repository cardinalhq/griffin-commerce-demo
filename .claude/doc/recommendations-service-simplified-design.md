# Simplified Recommendations Service System Design

## Overview
The Simplified Recommendations Service provides basic product recommendations for the Griffin Commerce demo. This is an EXTREMELY SIMPLIFIED design that returns random products without any machine learning, user profiling, or personalization.

**Core Principle**: Just return random products. That's it.

## System Architecture

### High-Level Components
```
┌─────────────────────────────────────────────────────────────┐
│              Simplified Recommendations Service            │
├─────────────────────────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐ │
│ │   HTTP Handlers │ │ Product Client  │ │ Random Selector │ │
│ │   (2 endpoints) │ │ (calls catalog) │ │   (shuffling)   │ │
│ └─────────────────┘ └─────────────────┘ └─────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

**What's REMOVED compared to complex version:**
- ❌ User profiles and tracking
- ❌ Machine learning algorithms
- ❌ Caching layers
- ❌ A/B testing framework
- ❌ Product affinity calculations
- ❌ Business rules engine
- ❌ Analytics and metrics beyond basic HTTP
- ❌ Complex configuration
- ❌ Multiple recommendation strategies

## File Structure (MINIMAL)
```
recommendations/
├── main.go           # Server setup and routing (50 lines)
├── handlers.go       # HTTP handlers (80 lines)
├── client.go         # Product service client (70 lines)
└── README.md         # Basic service info
```

**Total Lines of Code Target: < 200 lines**

## Data Models (Minimal)

### Product (from Product Service)
```go
type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
}
```

### Recommendation Response
```go
type RecommendationResponse struct {
    Recommendations []Product `json:"recommendations"`
}
```

**No other models needed!**

## API Specifications

### Port Configuration
- **Service Port**: 8085
- **Health Check**: GET /health

### Endpoints

#### 1. Product Recommendations
```
GET /api/recommendations/product/{id}
```

**Purpose**: Get 3 random products excluding the specified product

**Request Path Parameters:**
- `id` (string): Product ID to exclude from recommendations

**Response:**
```json
{
  "recommendations": [
    {"id": "PROD-002", "name": "Bacon Treats", "price": 9.99},
    {"id": "PROD-003", "name": "Tennis Ball", "price": 5.99},
    {"id": "PROD-004", "name": "Dog Bed", "price": 45.99}
  ]
}
```

**Error Cases:**
- 404: Product not found
- 500: Product service unavailable
- 200 with empty array: No other products available

#### 2. Cart Recommendations
```
GET /api/recommendations/cart/{id}
```

**Purpose**: Get 3 random products excluding items already in the cart

**Request Path Parameters:**
- `id` (string): Cart ID

**Response:**
```json
{
  "recommendations": [
    {"id": "PROD-005", "name": "Chew Rope", "price": 12.99},
    {"id": "PROD-006", "name": "Water Bowl", "price": 18.99},
    {"id": "PROD-007", "name": "Collar", "price": 25.99}
  ]
}
```

**Error Cases:**
- 404: Cart not found
- 500: Cart or Product service unavailable
- 200 with empty array: No products available to recommend

## Random Selection Logic

### Algorithm: Simple Random Selection
1. Call Product Service to get all products
2. Filter out excluded products (current product or cart items)
3. Shuffle remaining products using Go's `rand.Shuffle`
4. Return first 3 products (or fewer if less available)

### Implementation Pattern
```go
func getRandomProducts(allProducts []Product, excludeIDs []string, count int) []Product {
    // Filter out excluded products
    filtered := filterProducts(allProducts, excludeIDs)

    // Shuffle the filtered list
    rand.Shuffle(len(filtered), func(i, j int) {
        filtered[i], filtered[j] = filtered[j], filtered[i]
    })

    // Return first N products
    if len(filtered) < count {
        return filtered
    }
    return filtered[:count]
}
```

## Integration with Other Services

### Product Service Integration
**Base URL**: `http://localhost:8080`

#### Get All Products
```
GET /api/products
```

**Expected Response:**
```json
[
  {"id": "PROD-001", "name": "Premium Rope Toy", "price": 15.99},
  {"id": "PROD-002", "name": "Bacon Treats", "price": 9.99},
  {"id": "PROD-003", "name": "Tennis Ball", "price": 5.99}
]
```

### Cart Service Integration (Optional)
**Base URL**: `http://localhost:8082`

#### Get Cart Items
```
GET /api/cart/{id}
```

**Expected Response:**
```json
{
  "items": [
    {"productId": "PROD-001", "quantity": 1}
  ]
}
```

**Note**: If Cart Service is unavailable, cart recommendations will fall back to returning 3 random products.

## Component Specifications

### 1. main.go - Server Setup
**Responsibilities:**
- Initialize HTTP server on port 8085
- Set up basic routing
- Configure minimal logging
- Handle graceful shutdown

**Key Functions:**
- `main()` - Entry point
- `setupRoutes()` - Configure HTTP routes
- `health handler` - Basic health check

### 2. handlers.go - HTTP Handlers
**Responsibilities:**
- Handle product recommendation requests
- Handle cart recommendation requests
- Validate request parameters
- Return JSON responses
- Handle error cases gracefully

**Key Functions:**
- `GetProductRecommendations(w http.ResponseWriter, r *http.Request)`
- `GetCartRecommendations(w http.ResponseWriter, r *http.Request)`
- `writeJSONResponse(w http.ResponseWriter, data interface{})`
- `writeErrorResponse(w http.ResponseWriter, status int, message string)`

### 3. client.go - External Service Client
**Responsibilities:**
- Call Product Service API
- Call Cart Service API (with fallback)
- Handle network errors
- Parse JSON responses

**Key Functions:**
- `NewProductClient(baseURL string) *ProductClient`
- `GetAllProducts() ([]Product, error)`
- `GetCartItems(cartID string) ([]string, error)` // Returns product IDs
- `makeHTTPRequest(url string) ([]byte, error)`

## Testing Strategy

### Unit Tests Required

#### handlers_test.go
- Test product recommendations endpoint with valid product ID
- Test product recommendations endpoint with invalid product ID
- Test cart recommendations endpoint with valid cart ID
- Test cart recommendations endpoint with invalid cart ID
- Test error handling when Product Service is down
- Test JSON response formatting

#### client_test.go
- Test Product Service client with mock responses
- Test Cart Service client with mock responses
- Test network error handling
- Test JSON parsing errors
- Test timeout scenarios

#### Random Selection Tests
- Test filtering logic excludes correct products
- Test randomization produces different results across calls
- Test handling of empty product lists
- Test handling when exclusion list is larger than product list

### Integration Tests
- Test full request flow: HTTP → Product Service → Random Selection → Response
- Test service startup and health check
- Test graceful degradation when dependencies are unavailable

### Test Data Requirements
```go
var mockProducts = []Product{
    {ID: "PROD-001", Name: "Rope Toy", Price: 15.99},
    {ID: "PROD-002", Name: "Bacon Treats", Price: 9.99},
    {ID: "PROD-003", Name: "Tennis Ball", Price: 5.99},
    {ID: "PROD-004", Name: "Dog Bed", Price: 45.99},
    {ID: "PROD-005", Name: "Chew Bone", Price: 8.99},
}
```

## Implementation Order

### Phase 1: Basic Structure (30 minutes)
1. Create main.go with HTTP server setup
2. Create basic handlers.go with stub endpoints
3. Test server startup and health check

### Phase 2: Product Service Integration (45 minutes)
1. Implement client.go for Product Service calls
2. Implement GetAllProducts functionality
3. Test Product Service integration

### Phase 3: Random Selection Logic (30 minutes)
1. Implement product filtering logic
2. Implement random shuffling
3. Test random selection with various inputs

### Phase 4: Complete Endpoints (30 minutes)
1. Complete product recommendations endpoint
2. Complete cart recommendations endpoint (with fallback)
3. Add proper error handling

### Phase 5: Testing (15 minutes)
1. Add basic unit tests
2. Test integration with Product Service
3. Verify error cases work correctly

## Configuration

### Minimal Environment Variables
```bash
PORT=8085
PRODUCT_SERVICE_URL=http://localhost:8080
CART_SERVICE_URL=http://localhost:8082
LOG_LEVEL=info
```

### No YAML Configuration Needed
All configuration through environment variables to keep it simple.

## Deployment

### Docker Support (Optional)
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o recommendations ./main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /app/recommendations .
EXPOSE 8085
CMD ["./recommendations"]
```

### Local Development
```bash
# Run the service
go run main.go

# Test endpoints
curl http://localhost:8085/api/recommendations/product/PROD-001
curl http://localhost:8085/api/recommendations/cart/CART-123
```

## Error Handling Strategy

### Graceful Degradation
- If Product Service is unavailable: Return 500 error
- If Cart Service is unavailable: Fall back to random product selection
- If no products available: Return empty recommendations array
- If invalid IDs provided: Return 404 with helpful message

### HTTP Status Codes
- 200: Success (even with empty results)
- 404: Resource not found (invalid product/cart ID)
- 500: Service error (external service unavailable)

## Success Criteria

### Functional Requirements ✅
- Returns 3 random products excluding specified items
- Integrates with Product Service successfully
- Handles error cases gracefully
- Completes in under 100ms for normal requests

### Technical Requirements ✅
- Under 200 lines of total code
- No external dependencies beyond HTTP client
- Simple to understand and modify
- Easy to test and debug

### Business Requirements ✅
- Provides product discovery functionality
- Supports cart cross-selling scenarios
- Maintains performance under normal load
- Enables future enhancement if needed

## Future Considerations (OUT OF SCOPE)

This design intentionally excludes:
- User behavior tracking
- Recommendation personalization
- Product affinity algorithms
- A/B testing capabilities
- Caching mechanisms
- Advanced analytics
- Machine learning integration

If these features become required in the future, a complete redesign would be necessary rather than evolving this simple implementation.

## Monitoring and Observability

### Basic Metrics (via OpenTelemetry)
- HTTP request count and latency
- Product Service call success/failure rates
- Response size distribution

### Health Checks
- Service startup health
- Product Service connectivity

### Logging
- Request/response logging
- Error cases with context
- Service startup/shutdown events

**Note**: This is minimal observability compared to the complex recommendations service design.