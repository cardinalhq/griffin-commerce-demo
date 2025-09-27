# Simplified Product Catalog Service - System Design

## Overview

The Product Catalog Service is a simplified microservice that manages product information for the Griffin Commerce demo. This design prioritizes extreme simplicity and focuses on core functionality only, targeting a 1-day implementation timeline.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────┐
│                Product Catalog Service                  │
│                    Port: 8080                          │
├─────────────────────────────────────────────────────────┤
│  HTTP Layer                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   GET       │  │    POST     │  │    POST     │     │
│  │ /products   │  │ /reserve    │  │ /release    │     │
│  │ /products/id│  │             │  │             │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
├─────────────────────────────────────────────────────────┤
│  Business Logic Layer                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Product    │  │   Stock     │  │   YAML      │     │
│  │ Operations  │  │ Management  │  │   Loader    │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
├─────────────────────────────────────────────────────────┤
│  Data Layer (In-Memory)                                 │
│  ┌─────────────────────────────────────────────────┐   │
│  │           Common Service MockDB                 │   │
│  │        map[string]*Product + sync.RWMutex       │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
           │                                │
           │ Startup                        │ Runtime
           ▼                                ▼
    ┌─────────────┐                ┌─────────────┐
    │products.yaml│                │   Other     │
    │   (Static   │                │ Services    │
    │  Products)  │                │ (Cart, etc) │
    └─────────────┘                └─────────────┘
```

## File Structure

```
catalog/
├── main.go           # Server setup and startup
├── handlers.go       # HTTP request handlers
├── products.go       # Product business logic
└── products.yaml     # Initial product data
```

## Product Model

### Data Structure
```go
type Product struct {
    ID       string  `json:"id" yaml:"id"`
    Name     string  `json:"name" yaml:"name"`
    Price    float64 `json:"price" yaml:"price"`
    Stock    int     `json:"stock" yaml:"stock"`
    Category string  `json:"category" yaml:"category"`
    ImageURL string  `json:"image_url" yaml:"image_url"`
}
```

### YAML Configuration (products.yaml)
```yaml
products:
  - id: "PROD-001"
    name: "Premium Rope Toy"
    price: 15.99
    stock: 100
    category: "toys"
    image_url: "/static/rope-toy.jpg"
  - id: "PROD-002"
    name: "Bacon Treats"
    price: 9.99
    stock: 250
    category: "treats"
    image_url: "/static/bacon-treats.jpg"
  - id: "PROD-003"
    name: "Squeaky Ball"
    price: 8.99
    stock: 75
    category: "toys"
    image_url: "/static/squeaky-ball.jpg"
  - id: "PROD-004"
    name: "Dog Collar"
    price: 24.99
    stock: 50
    category: "accessories"
    image_url: "/static/dog-collar.jpg"
```

## API Specifications

### 1. List All Products
```
GET /api/products
```

**Response:**
```json
{
  "products": [
    {
      "id": "PROD-001",
      "name": "Premium Rope Toy",
      "price": 15.99,
      "stock": 100,
      "category": "toys",
      "image_url": "/static/rope-toy.jpg"
    }
  ]
}
```

**Status Codes:**
- 200: Success
- 500: Internal server error

### 2. Get Single Product
```
GET /api/products/{id}
```

**Response (Success):**
```json
{
  "id": "PROD-001",
  "name": "Premium Rope Toy",
  "price": 15.99,
  "stock": 100,
  "category": "toys",
  "image_url": "/static/rope-toy.jpg"
}
```

**Response (Not Found):**
```json
{
  "error": "product not found",
  "code": "PRODUCT_NOT_FOUND"
}
```

**Status Codes:**
- 200: Product found
- 404: Product not found
- 500: Internal server error

### 3. Reserve Stock
```
POST /api/products/{id}/reserve
Content-Type: application/json

{
  "quantity": 2
}
```

**Response (Success):**
```json
{
  "id": "PROD-001",
  "reserved_quantity": 2,
  "remaining_stock": 98
}
```

**Response (Insufficient Stock):**
```json
{
  "error": "insufficient stock",
  "code": "INSUFFICIENT_STOCK",
  "available": 1,
  "requested": 2
}
```

**Status Codes:**
- 200: Stock reserved successfully
- 400: Insufficient stock or invalid quantity
- 404: Product not found
- 500: Internal server error

### 4. Release Stock
```
POST /api/products/{id}/release
Content-Type: application/json

{
  "quantity": 1
}
```

**Response:**
```json
{
  "id": "PROD-001",
  "released_quantity": 1,
  "new_stock": 99
}
```

**Status Codes:**
- 200: Stock released successfully
- 400: Invalid quantity
- 404: Product not found
- 500: Internal server error

## Stock Management

### Thread-Safe Operations

Stock operations use atomic operations to ensure thread safety:

```go
type StockManager struct {
    products map[string]*Product
    mu       sync.RWMutex
}

func (sm *StockManager) ReserveStock(productID string, quantity int) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    product, exists := sm.products[productID]
    if !exists {
        return ErrProductNotFound
    }

    if product.Stock < quantity {
        return ErrInsufficientStock
    }

    product.Stock -= quantity
    return nil
}

func (sm *StockManager) ReleaseStock(productID string, quantity int) error {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    product, exists := sm.products[productID]
    if !exists {
        return ErrProductNotFound
    }

    product.Stock += quantity
    return nil
}
```

### Stock Operation Rules

1. **Reserve Stock**: Decrements available stock atomically
2. **Release Stock**: Increments available stock atomically
3. **No Negative Stock**: Reserve operations fail if insufficient stock
4. **No Stock Tracking**: No audit trail or reservation records
5. **Immediate Effect**: All changes are immediate (no commit/rollback)

## Integration with Common Service

### MockDB Usage

```go
// Initialize using Common Service MockDB
db := common.NewMockDB()

// Store products in MockDB
for _, product := range products {
    db.Set("product:"+product.ID, product)
}

// Retrieve products
product := &Product{}
err := db.Get("product:"+productID, product)
```

### HTTP Middleware

The service uses Common Service middleware:

```go
import "github.com/cardinalhq/griffin-commerce-demo/common"

func setupServer() *http.Server {
    mux := http.NewServeMux()

    // Apply common middleware
    handler := common.WithLogging(
        common.WithTracing(
            common.WithCorrelationID(mux),
        ),
    )

    return &http.Server{
        Addr:    ":8080",
        Handler: handler,
    }
}
```

### Error Types

Uses Common Service error types:

```go
import "github.com/cardinalhq/griffin-commerce-demo/common"

var (
    ErrProductNotFound   = common.NewError("PRODUCT_NOT_FOUND", "product not found")
    ErrInsufficientStock = common.NewError("INSUFFICIENT_STOCK", "insufficient stock")
    ErrInvalidQuantity   = common.NewError("INVALID_QUANTITY", "quantity must be positive")
)
```

## Startup Sequence

### 1. Configuration Loading
```go
func main() {
    // Load configuration
    config := common.LoadConfig("config.yaml")

    // Setup telemetry
    tracer := common.SetupTelemetry(config.ServiceName)
    defer tracer.Shutdown()
}
```

### 2. Product Data Loading
```go
func loadProducts() ([]*Product, error) {
    data, err := os.ReadFile("products.yaml")
    if err != nil {
        return nil, fmt.Errorf("failed to read products.yaml: %w", err)
    }

    var config struct {
        Products []*Product `yaml:"products"`
    }

    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("failed to parse products.yaml: %w", err)
    }

    return config.Products, nil
}
```

### 3. Database Initialization
```go
func initializeDatabase(products []*Product) *common.MockDB {
    db := common.NewMockDB()

    for _, product := range products {
        db.Set("product:"+product.ID, product)
    }

    log.Printf("Loaded %d products into memory", len(products))
    return db
}
```

### 4. Server Startup
```go
func main() {
    // 1. Load products from YAML
    products, err := loadProducts()
    if err != nil {
        log.Fatal("Failed to load products:", err)
    }

    // 2. Initialize database
    db := initializeDatabase(products)

    // 3. Setup handlers
    setupHandlers(db)

    // 4. Start server
    log.Println("Product Catalog Service starting on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal("Server failed:", err)
    }
}
```

## Component Specifications

### 1. main.go (Server Setup)

**Responsibilities:**
- Load products from YAML file
- Initialize MockDB with product data
- Setup HTTP server and middleware
- Configure OpenTelemetry
- Handle graceful shutdown

**Key Functions:**
- `main()`: Entry point and orchestration
- `loadProducts()`: YAML parsing and validation
- `initializeDatabase()`: MockDB setup
- `setupServer()`: HTTP server configuration

### 2. handlers.go (HTTP Handlers)

**Responsibilities:**
- Handle HTTP requests and responses
- Request validation and parsing
- Error handling and status codes
- Response formatting

**Key Functions:**
- `listProductsHandler()`: GET /api/products
- `getProductHandler()`: GET /api/products/{id}
- `reserveStockHandler()`: POST /api/products/{id}/reserve
- `releaseStockHandler()`: POST /api/products/{id}/release

**Handler Constraints:**
- Each handler under 50 lines
- Common error handling pattern
- JSON request/response only
- No complex business logic

### 3. products.go (Business Logic)

**Responsibilities:**
- Product CRUD operations
- Stock management logic
- Data validation
- Business rule enforcement

**Key Functions:**
- `GetAllProducts()`: Retrieve all products
- `GetProduct()`: Retrieve single product
- `ReserveStock()`: Atomic stock decrement
- `ReleaseStock()`: Atomic stock increment
- `ValidateQuantity()`: Input validation

## Testing Strategy

### Unit Tests Required

#### 1. Product Loading Tests
```go
func TestLoadProducts(t *testing.T) {
    // Test valid YAML loading
    // Test invalid YAML handling
    // Test missing file handling
    // Test empty products list
}
```

#### 2. Stock Management Tests
```go
func TestReserveStock(t *testing.T) {
    // Test successful reservation
    // Test insufficient stock
    // Test product not found
    // Test invalid quantity
    // Test concurrent access
}

func TestReleaseStock(t *testing.T) {
    // Test successful release
    // Test product not found
    // Test invalid quantity
}
```

#### 3. Handler Tests
```go
func TestListProductsHandler(t *testing.T) {
    // Test successful response
    // Test empty product list
    // Test server error
}

func TestGetProductHandler(t *testing.T) {
    // Test product found
    // Test product not found
    // Test invalid product ID
}

func TestReserveStockHandler(t *testing.T) {
    // Test successful reservation
    // Test insufficient stock
    // Test invalid JSON
    // Test missing quantity
    // Test negative quantity
}
```

#### 4. Concurrency Tests
```go
func TestConcurrentStockOperations(t *testing.T) {
    // Test multiple goroutines reserving stock
    // Test race condition prevention
    // Test stock consistency
}
```

### Test Data
```yaml
# test_products.yaml
products:
  - id: "TEST-001"
    name: "Test Product"
    price: 10.00
    stock: 5
    category: "test"
    image_url: "/static/test.jpg"
```

## Implementation Order

### Phase 1: Core Structure (2-4 hours)
1. Create basic file structure
2. Implement Product struct and YAML loading
3. Setup MockDB integration
4. Create basic HTTP server

### Phase 2: API Implementation (3-4 hours)
1. Implement GET /api/products handler
2. Implement GET /api/products/{id} handler
3. Add error handling and response formatting
4. Test basic functionality

### Phase 3: Stock Operations (2-3 hours)
1. Implement stock reserve handler
2. Implement stock release handler
3. Add thread-safety mechanisms
4. Test concurrent operations

### Phase 4: Integration & Testing (1-2 hours)
1. Add Common Service middleware
2. Setup OpenTelemetry tracing
3. Write unit tests
4. Integration testing with other services

## Dependencies

### External Dependencies
```go
// go.mod
module github.com/cardinalhq/griffin-commerce-demo/catalog

go 1.21

require (
    github.com/cardinalhq/griffin-commerce-demo/common v0.0.0
    gopkg.in/yaml.v2 v2.4.0
    go.opentelemetry.io/otel v1.19.0
    go.opentelemetry.io/otel/trace v1.19.0
)
```

### Internal Dependencies
- Common Service: MockDB, middleware, error types
- Configuration: Shared config.yaml
- Products YAML: Static product data

## Configuration

The service reads from the shared config.yaml:

```yaml
services:
  catalog:
    port: 8080
    products_file: "products.yaml"

telemetry:
  enabled: true
  service_name: "product-catalog"
```

## Monitoring and Observability

### OpenTelemetry Tracing
- HTTP request spans automatically created
- Custom spans for:
  - Product loading from YAML
  - Stock operations
  - Database operations

### Logging
- Startup/shutdown events
- Product loading results
- Stock operation outcomes
- Error conditions

### Health Check
```
GET /health
Response: {"status": "healthy", "products_loaded": 4}
```

## Success Criteria

1. **Functionality**: All API endpoints work as specified
2. **Performance**: Sub-10ms response times for all operations
3. **Reliability**: Thread-safe stock operations under concurrent load
4. **Observability**: Request tracing works end-to-end
5. **Simplicity**: Total implementation under 500 lines of code
6. **Integration**: Successfully integrates with Common Service
7. **Testing**: 90%+ code coverage with unit tests

## Constraints and Limitations

### Intentional Limitations
- No search or filtering capabilities
- No product creation/updates via API
- No persistence beyond memory
- No caching layer
- No complex inventory management
- No product variants or attributes

### Technical Constraints
- Single process, no clustering
- Data lost on restart
- No backup/recovery mechanisms
- No audit trail for stock changes
- Basic error handling only

This simplified design achieves the core requirements while maintaining extreme simplicity and enabling rapid implementation within the 1-day target timeline.