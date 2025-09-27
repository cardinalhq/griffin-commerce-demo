# Simplified Shopping Cart Service - System Design

## 1. System Overview

The Simplified Shopping Cart Service is designed for the Griffin Commerce POC to demonstrate basic cart operations with extreme simplicity. This design prioritizes implementation speed and ease of understanding over feature completeness.

### High-Level Architecture

```
┌─────────────────┐
│    Frontend     │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Cart Service   │    ───►  Product Service (validate products)
│   (Port 8082)   │
│                 │    ───►  Common Service (MockDB, middleware)
└─────────────────┘
```

### Design Principles
- **Extreme Simplicity**: Only essential cart operations
- **Flat Structure**: Minimal directory nesting
- **In-Memory Storage**: No persistence beyond session
- **Direct Calculation**: Simple sum(price × quantity)
- **No Sessions**: Every cart requires a CustomerID

## 2. File Structure (FLAT)

```
cart/
├── main.go      # Server setup (HTTP server, config, routes)
├── handlers.go  # HTTP handlers (all endpoints)
├── cart.go      # Cart business logic and models
└── client.go    # Product service client
```

**Total Files: 4** (implementation goal: 1 day)

## 3. Data Models

### Simplified Cart Model

```go
type Cart struct {
    ID         string     `json:"id"`
    CustomerID string     `json:"customer_id"`  // Always required
    Items      []CartItem `json:"items"`
    Total      float64    `json:"total"`
    CreatedAt  time.Time  `json:"created_at"`
}

type CartItem struct {
    ProductID string  `json:"product_id"`
    Quantity  int     `json:"quantity"`
    Price     float64 `json:"price"`
}
```

**No Complex Models:**
- No sessions (CustomerID required)
- No promotions
- No tax/shipping calculations
- No cart status/metadata

## 4. API Design

### Base URL: `/api/cart`

#### 4.1 Create Cart
```
POST /api/cart/create
```

**Request Body:**
```json
{
  "customer_id": "cust_123"
}
```

**Response (201):**
```json
{
  "id": "cart_abc123",
  "customer_id": "cust_123",
  "items": [],
  "total": 0.00,
  "created_at": "2025-01-01T00:00:00Z"
}
```

#### 4.2 Get Cart
```
GET /api/cart/{id}
```

**Response (200):**
```json
{
  "id": "cart_abc123",
  "customer_id": "cust_123",
  "items": [
    {
      "product_id": "PROD-001",
      "quantity": 2,
      "price": 15.99
    }
  ],
  "total": 31.98,
  "created_at": "2025-01-01T00:00:00Z"
}
```

#### 4.3 Add Item
```
POST /api/cart/{id}/add
```

**Request Body:**
```json
{
  "product_id": "PROD-001",
  "quantity": 1
}
```

**Response (200):**
```json
{
  "id": "cart_abc123",
  "customer_id": "cust_123",
  "items": [
    {
      "product_id": "PROD-001",
      "quantity": 1,
      "price": 15.99
    }
  ],
  "total": 15.99,
  "created_at": "2025-01-01T00:00:00Z"
}
```

#### 4.4 Remove Item
```
DELETE /api/cart/{id}/item/{productId}
```

**Response (200):** Updated cart JSON

#### 4.5 Checkout (Begin)
```
POST /api/cart/{id}/checkout
```

**Response (200):**
```json
{
  "cart_id": "cart_abc123",
  "items": [...],
  "total": 31.98,
  "status": "ready_for_checkout"
}
```

### Error Responses

**Standard Error Format:**
```json
{
  "error": "CART_NOT_FOUND",
  "message": "Cart with ID cart_abc123 not found"
}
```

**Error Codes:**
- `CART_NOT_FOUND` (404)
- `PRODUCT_NOT_FOUND` (404)
- `INVALID_QUANTITY` (400)
- `CUSTOMER_ID_REQUIRED` (400)

## 5. Business Logic

### Cart Operations

```go
type CartService struct {
    carts     map[string]*Cart  // In-memory storage
    mutex     sync.RWMutex      // Thread safety
    productClient ProductClient
}

func (cs *CartService) CreateCart(customerID string) (*Cart, error) {
    // 1. Validate customerID is not empty
    // 2. Generate cart ID
    // 3. Create cart with empty items
    // 4. Store in memory map
    // 5. Return cart
}

func (cs *CartService) AddItem(cartID, productID string, quantity int) (*Cart, error) {
    // 1. Get cart from memory
    // 2. Validate product exists (call Product Service)
    // 3. Check if item already in cart (update quantity vs add new)
    // 4. Calculate new total
    // 5. Update cart in memory
    // 6. Return updated cart
}

func (cs *CartService) RemoveItem(cartID, productID string) (*Cart, error) {
    // 1. Get cart from memory
    // 2. Find and remove item
    // 3. Recalculate total
    // 4. Update cart in memory
    // 5. Return updated cart
}

func (cs *CartService) CalculateTotal(cart *Cart) float64 {
    total := 0.0
    for _, item := range cart.Items {
        total += item.Price * float64(item.Quantity)
    }
    return total
}
```

### Business Rules (Minimal)
- CustomerID is always required (no guest carts)
- Maximum 10 items per cart
- Maximum quantity 10 per item
- Duplicate products update quantity (don't add new line item)

## 6. Product Service Integration

### Product Client

```go
type ProductClient struct {
    baseURL string
    client  *http.Client
}

type Product struct {
    ID    string  `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
    Stock int     `json:"stock"`
}

func (pc *ProductClient) GetProduct(productID string) (*Product, error) {
    // GET http://localhost:8080/api/products/{productID}
    // Return product details or error if not found
}

func (pc *ProductClient) ValidateProduct(productID string) (bool, float64, error) {
    product, err := pc.GetProduct(productID)
    if err != nil {
        return false, 0, err
    }
    return true, product.Price, nil
}
```

### Integration Points
- **Product Validation**: Call Product Service to verify product exists
- **Price Retrieval**: Get current price when adding to cart
- **No Inventory Check**: Simplified - assume products are available

## 7. Error Handling

### Error Strategy
- **Simple Errors**: Return basic error messages
- **No Retries**: If Product Service fails, return error immediately
- **No Circuit Breakers**: Keep it simple
- **HTTP Status Codes**: Standard REST status codes

```go
type CartError struct {
    Code    string `json:"error"`
    Message string `json:"message"`
}

var (
    ErrCartNotFound     = CartError{"CART_NOT_FOUND", "Cart not found"}
    ErrProductNotFound  = CartError{"PRODUCT_NOT_FOUND", "Product not found"}
    ErrInvalidQuantity  = CartError{"INVALID_QUANTITY", "Quantity must be between 1 and 10"}
    ErrCustomerRequired = CartError{"CUSTOMER_ID_REQUIRED", "Customer ID is required"}
    ErrCartFull         = CartError{"CART_FULL", "Cart cannot exceed 10 items"}
)
```

## 8. Testing Strategy

### Unit Tests (Per File)

**cart_test.go:**
```go
func TestCartService_CreateCart(t *testing.T) {
    // Test successful cart creation
    // Test empty customer ID validation
}

func TestCartService_AddItem(t *testing.T) {
    // Test adding new item
    // Test updating existing item quantity
    // Test product validation failure
    // Test cart item limit
}

func TestCartService_CalculateTotal(t *testing.T) {
    // Test empty cart (0.00)
    // Test single item
    // Test multiple items
    // Test quantity > 1
}
```

**handlers_test.go:**
```go
func TestCreateCartHandler(t *testing.T) {
    // Test successful creation
    // Test missing customer_id
    // Test malformed JSON
}

func TestAddItemHandler(t *testing.T) {
    // Test successful add
    // Test cart not found
    // Test product not found
}
```

### Integration Tests

**API End-to-End Test:**
```go
func TestCartWorkflow(t *testing.T) {
    // 1. Create cart
    // 2. Add multiple items
    // 3. Remove an item
    // 4. Verify totals
    // 5. Begin checkout
}
```

## 9. Implementation Plan

### Implementation Order (1 Day)

**Morning (4 hours):**
1. **main.go** - Setup HTTP server, config, routes (1 hour)
2. **cart.go** - Data models, CartService, business logic (2 hours)
3. **client.go** - Product service client (1 hour)

**Afternoon (4 hours):**
1. **handlers.go** - All HTTP handlers (2 hours)
2. **Unit tests** - Test core logic (1 hour)
3. **Integration testing** - Test with Product Service (1 hour)

### Dependencies
- **Common Service**: MockDB for storage, middleware for CORS/logging
- **Product Service**: Running on port 8080 for product validation

## 10. Configuration

### Minimal Configuration

```yaml
# config.yaml (embedded in main.go)
cart_service:
  port: 8082
  max_items_per_cart: 10
  max_quantity_per_item: 10

product_service:
  url: "http://localhost:8080"
  timeout: "5s"
```

## 11. No-Frills Features

### What's NOT Included (to maintain simplicity)
- ❌ Sessions or session management
- ❌ Guest carts or cart merging
- ❌ Promotions, discounts, or coupon codes
- ❌ Tax or shipping calculations
- ❌ Cart persistence (memory only)
- ❌ Cart expiration or cleanup
- ❌ Complex error handling or retries
- ❌ Metrics or detailed observability
- ❌ Cart reservation during checkout
- ❌ Middleware beyond basic CORS
- ❌ Validation beyond basic checks
- ❌ Caching of any kind

### What IS Included (minimal viable cart)
- ✅ Create cart for customer
- ✅ Add/remove items
- ✅ Simple total calculation
- ✅ Product validation via Product Service
- ✅ Basic error handling
- ✅ RESTful API endpoints
- ✅ In-memory storage
- ✅ Unit and integration tests

## 12. Sample Implementation Flow

### Adding an Item Flow
```
1. POST /api/cart/cart_123/add { product_id: "PROD-001", quantity: 2 }
2. Handler validates request JSON
3. CartService.AddItem() called
4. Product client validates PROD-001 exists and gets price
5. Check if PROD-001 already in cart
   - If yes: Update quantity (existing + new)
   - If no: Add new cart item
6. Recalculate total (sum all item.Price * item.Quantity)
7. Update cart in memory map
8. Return updated cart JSON
```

### Error Flow
```
1. Product Service call fails
2. Return 404 with { "error": "PRODUCT_NOT_FOUND", "message": "..." }
3. No retries, no complex error handling
```

## 13. Success Criteria

### Functional Success
- ✅ Cart CRUD operations work
- ✅ Items can be added/removed
- ✅ Totals calculate correctly
- ✅ Product validation works
- ✅ All API endpoints respond correctly

### Performance Success
- ✅ All operations complete under 100ms
- ✅ Can handle 100 concurrent operations
- ✅ Memory usage stays reasonable

### Implementation Success
- ✅ Complete implementation in 1 day
- ✅ Code is simple and readable
- ✅ Tests pass and provide good coverage
- ✅ Integrates properly with Product Service

This simplified design eliminates all complexity while maintaining the core cart functionality needed for the POC. The flat file structure and minimal business logic make it implementable in a single day while still demonstrating microservice communication patterns.