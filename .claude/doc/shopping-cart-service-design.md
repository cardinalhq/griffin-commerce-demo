# Shopping Cart Service - System Design

## 1. System Overview

The Shopping Cart Service is a core component of the Griffin Commerce demo e-commerce platform that manages customer shopping sessions, cart operations, and cart persistence. It supports both guest and authenticated users with seamless cart merging capabilities.

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │    │  Frontend/Web   │    │  Mobile Apps    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
         ┌───────────────────────▼───────────────────────┐
         │            Shopping Cart Service              │
         │  ┌─────────────────────────────────────────┐  │
         │  │           API Layer                     │  │
         │  └─────────────────────────────────────────┘  │
         │  ┌─────────────────────────────────────────┐  │
         │  │         Business Logic Layer            │  │
         │  │  ┌─────────┐ ┌─────────┐ ┌─────────────┐ │  │
         │  │  │Cart Mgmt│ │Sessions │ │Calculations │ │  │
         │  │  └─────────┘ └─────────┘ └─────────────┘ │  │
         │  └─────────────────────────────────────────┘  │
         │  ┌─────────────────────────────────────────┐  │
         │  │        Data Access Layer                │  │
         │  └─────────────────────────────────────────┘  │
         │  ┌─────────────────────────────────────────┐  │
         │  │         Mock Database                   │  │
         │  └─────────────────────────────────────────┘  │
         └─────────────────────────────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
    ┌─────▼──────┐    ┌─────▼──────┐    ┌─────▼──────┐
    │  Product   │    │ Promotion  │    │ Inventory  │
    │  Catalog   │    │  Service   │    │  Service   │
    └────────────┘    └────────────┘    └────────────┘
```

### Key Components

1. **API Layer**: REST endpoints for cart operations
2. **Session Manager**: Handles guest and authenticated user sessions
3. **Cart Manager**: Core cart operations (CRUD, calculations)
4. **Promotion Engine**: Discount and promo code processing
5. **Mock Database**: In-memory storage with optional JSON persistence
6. **Integration Clients**: Communication with external services

## 2. File Structure

```
cmd/cart/
├── main.go                           # Service entry point
├── config/
│   ├── config.go                     # Configuration management
│   └── config.yaml                   # Service configuration
pkg/cart/
├── api/
│   ├── handlers.go                   # HTTP request handlers
│   ├── middleware.go                 # HTTP middleware (auth, logging, tracing)
│   ├── routes.go                     # Route definitions
│   └── validation.go                 # Request validation
├── domain/
│   ├── cart.go                       # Cart domain models
│   ├── session.go                    # Session domain models
│   ├── promotion.go                  # Promotion domain models
│   └── errors.go                     # Domain-specific errors
├── service/
│   ├── cart_service.go               # Core cart business logic
│   ├── session_service.go            # Session management
│   ├── calculation_service.go        # Cart calculations
│   └── promotion_service.go          # Promotion application
├── repository/
│   ├── interfaces.go                 # Repository interfaces
│   ├── mock_cart_repository.go       # Mock cart repository
│   └── mock_session_repository.go    # Mock session repository
├── client/
│   ├── product_client.go             # Product service client
│   ├── inventory_client.go           # Inventory service client
│   └── promotion_client.go           # Promotion service client
├── mockdb/
│   ├── database.go                   # Mock database implementation
│   ├── cart_store.go                 # Cart data store
│   └── session_store.go              # Session data store
├── telemetry/
│   ├── tracing.go                    # OpenTelemetry tracing
│   ├── metrics.go                    # Metrics collection
│   └── logging.go                    # Structured logging
└── fault/
    ├── injector.go                   # Fault injection framework
    └── config.go                     # Fault injection configuration
test/
├── integration/
│   ├── cart_api_test.go              # API integration tests
│   └── cart_scenarios_test.go        # Business scenario tests
├── unit/
│   ├── cart_service_test.go          # Unit tests for cart service
│   ├── session_service_test.go       # Unit tests for session service
│   └── calculation_service_test.go   # Unit tests for calculations
└── testdata/
    ├── test_scenarios.yaml           # Test data scenarios
    └── mock_responses.yaml           # Mock service responses
```

## 3. Data Models

### Core Domain Models

```go
// Cart represents a shopping cart
type Cart struct {
    ID          string                 `json:"id"`
    CustomerID  *string               `json:"customer_id,omitempty"`
    SessionID   string                `json:"session_id"`
    Status      CartStatus            `json:"status"`
    Items       []CartItem            `json:"items"`
    Promotions  []CartPromotion       `json:"promotions"`
    Totals      CartTotals            `json:"totals"`
    CreatedAt   time.Time             `json:"created_at"`
    UpdatedAt   time.Time             `json:"updated_at"`
    ExpiresAt   time.Time             `json:"expires_at"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CartItem represents an item in the cart
type CartItem struct {
    ID           string    `json:"id"`
    ProductID    string    `json:"product_id"`
    ProductSKU   string    `json:"product_sku"`
    ProductName  string    `json:"product_name"`
    Quantity     int       `json:"quantity"`
    PriceAtAdd   float64   `json:"price_at_add"`
    CurrentPrice float64   `json:"current_price"`
    AddedAt      time.Time `json:"added_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// CartPromotion represents applied promotions
type CartPromotion struct {
    ID             string          `json:"id"`
    PromoCode      string          `json:"promo_code"`
    DiscountAmount float64         `json:"discount_amount"`
    DiscountType   DiscountType    `json:"discount_type"`
    AppliedAt      time.Time       `json:"applied_at"`
}

// CartTotals represents calculated cart totals
type CartTotals struct {
    Subtotal     float64 `json:"subtotal"`
    Tax          float64 `json:"tax"`
    Shipping     float64 `json:"shipping"`
    Discount     float64 `json:"discount"`
    Total        float64 `json:"total"`
    Currency     string  `json:"currency"`
    CalculatedAt time.Time `json:"calculated_at"`
}

// Session represents a user session
type Session struct {
    ID          string                 `json:"id"`
    CustomerID  *string               `json:"customer_id,omitempty"`
    CartID      *string               `json:"cart_id,omitempty"`
    IsGuest     bool                  `json:"is_guest"`
    CreatedAt   time.Time             `json:"created_at"`
    LastAccess  time.Time             `json:"last_access"`
    ExpiresAt   time.Time             `json:"expires_at"`
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Enums
type CartStatus string
const (
    CartStatusActive    CartStatus = "active"
    CartStatusAbandoned CartStatus = "abandoned"
    CartStatusCheckedOut CartStatus = "checked_out"
    CartStatusExpired   CartStatus = "expired"
)

type DiscountType string
const (
    DiscountTypeFixed      DiscountType = "fixed"
    DiscountTypePercentage DiscountType = "percentage"
    DiscountTypeFreeShipping DiscountType = "free_shipping"
)
```

### Mock Database Schema

```go
// MockDatabase structure for JSON persistence
type MockDatabase struct {
    Carts    map[string]*Cart    `json:"carts"`
    Sessions map[string]*Session `json:"sessions"`
    Metadata DatabaseMetadata    `json:"metadata"`
}

type DatabaseMetadata struct {
    Version     string    `json:"version"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    TotalCarts  int       `json:"total_carts"`
    ActiveCarts int       `json:"active_carts"`
}
```

## 4. API Design

### Base URL
`/api/v1/cart`

### Endpoints

#### 4.1 Create Cart
```
POST /api/v1/cart/create
```

**Request Body:**
```json
{
  "session_id": "sess_123456789",
  "customer_id": "cust_123456789" // optional
}
```

**Response (201):**
```json
{
  "id": "cart_123456789",
  "session_id": "sess_123456789",
  "customer_id": "cust_123456789",
  "status": "active",
  "items": [],
  "promotions": [],
  "totals": {
    "subtotal": 0.00,
    "tax": 0.00,
    "shipping": 0.00,
    "discount": 0.00,
    "total": 0.00,
    "currency": "USD",
    "calculated_at": "2025-01-01T00:00:00Z"
  },
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z",
  "expires_at": "2025-01-31T00:00:00Z"
}
```

#### 4.2 Get Cart
```
GET /api/v1/cart/{cartId}
```

**Response (200):**
```json
{
  "id": "cart_123456789",
  "session_id": "sess_123456789",
  "customer_id": "cust_123456789",
  "status": "active",
  "items": [
    {
      "id": "item_123456789",
      "product_id": "DOG-TOY-001",
      "product_sku": "ROPE-TOY-LG",
      "product_name": "Premium Rope Toy - Large",
      "quantity": 2,
      "price_at_add": 15.99,
      "current_price": 15.99,
      "added_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-01T00:00:00Z"
    }
  ],
  "promotions": [],
  "totals": {
    "subtotal": 31.98,
    "tax": 2.56,
    "shipping": 5.99,
    "discount": 0.00,
    "total": 40.53,
    "currency": "USD",
    "calculated_at": "2025-01-01T00:00:00Z"
  },
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-01T00:00:00Z",
  "expires_at": "2025-01-31T00:00:00Z"
}
```

#### 4.3 Add Item to Cart
```
POST /api/v1/cart/{cartId}/items
```

**Request Body:**
```json
{
  "product_id": "DOG-TOY-001",
  "product_sku": "ROPE-TOY-LG",
  "quantity": 1
}
```

**Response (200):**
```json
{
  "item": {
    "id": "item_123456789",
    "product_id": "DOG-TOY-001",
    "product_sku": "ROPE-TOY-LG",
    "product_name": "Premium Rope Toy - Large",
    "quantity": 1,
    "price_at_add": 15.99,
    "current_price": 15.99,
    "added_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
  },
  "cart_totals": {
    "subtotal": 15.99,
    "tax": 1.28,
    "shipping": 5.99,
    "discount": 0.00,
    "total": 23.26,
    "currency": "USD",
    "calculated_at": "2025-01-01T00:00:00Z"
  }
}
```

#### 4.4 Update Item Quantity
```
PUT /api/v1/cart/{cartId}/items/{itemId}
```

**Request Body:**
```json
{
  "quantity": 3
}
```

#### 4.5 Remove Item from Cart
```
DELETE /api/v1/cart/{cartId}/items/{itemId}
```

#### 4.6 Apply Promotion
```
POST /api/v1/cart/{cartId}/apply-promo
```

**Request Body:**
```json
{
  "promo_code": "SAVE10"
}
```

**Response (200):**
```json
{
  "promotion": {
    "id": "promo_123456789",
    "promo_code": "SAVE10",
    "discount_amount": 5.00,
    "discount_type": "fixed",
    "applied_at": "2025-01-01T00:00:00Z"
  },
  "cart_totals": {
    "subtotal": 31.98,
    "tax": 2.56,
    "shipping": 5.99,
    "discount": 5.00,
    "total": 35.53,
    "currency": "USD",
    "calculated_at": "2025-01-01T00:00:00Z"
  }
}
```

#### 4.7 Merge Carts
```
POST /api/v1/cart/merge
```

**Request Body:**
```json
{
  "source_cart_id": "cart_guest_123",
  "target_cart_id": "cart_user_456",
  "customer_id": "cust_123456789"
}
```

#### 4.8 Initialize Checkout
```
POST /api/v1/cart/{cartId}/checkout
```

**Response (200):**
```json
{
  "checkout_session_id": "checkout_123456789",
  "cart_id": "cart_123456789",
  "reserved_until": "2025-01-01T00:15:00Z",
  "totals": {
    "subtotal": 31.98,
    "tax": 2.56,
    "shipping": 5.99,
    "discount": 0.00,
    "total": 40.53,
    "currency": "USD"
  }
}
```

### Error Responses

```json
{
  "error": {
    "code": "CART_NOT_FOUND",
    "message": "Cart with ID cart_123456789 not found",
    "details": {
      "cart_id": "cart_123456789"
    },
    "trace_id": "trace_123456789"
  }
}
```

**Error Codes:**
- `CART_NOT_FOUND` (404)
- `CART_EXPIRED` (410)
- `CART_ITEM_NOT_FOUND` (404)
- `PRODUCT_NOT_AVAILABLE` (409)
- `QUANTITY_LIMIT_EXCEEDED` (422)
- `CART_ITEMS_LIMIT_EXCEEDED` (422)
- `PROMO_CODE_INVALID` (422)
- `PROMO_CODE_EXPIRED` (422)
- `SESSION_INVALID` (401)
- `VALIDATION_ERROR` (400)
- `INTERNAL_ERROR` (500)

## 5. Session Management Strategy

### Session Types

1. **Guest Sessions**
   - Generated on first cart creation
   - Stored in cookies/local storage
   - 30-day expiration
   - Associated with IP address for basic fraud detection

2. **Authenticated Sessions**
   - Linked to customer ID
   - Can have multiple active sessions (multi-device)
   - Extended expiration (30 days of inactivity)
   - Session merging capabilities

### Session Flow

```
Guest User Flow:
1. User visits site → Generate session ID
2. Add items to cart → Create cart with session ID
3. User logs in → Merge guest cart with user cart
4. Continue as authenticated user

Authenticated User Flow:
1. User logs in → Create/retrieve session
2. Load existing cart or create new cart
3. Continue with cart operations
4. Maintain session across devices
```

### Session Storage

```go
type SessionStore struct {
    sessions map[string]*Session
    mutex    sync.RWMutex
    config   SessionConfig
}

type SessionConfig struct {
    GuestSessionTTL   time.Duration
    UserSessionTTL    time.Duration
    CleanupInterval   time.Duration
    MaxSessionsPerUser int
}
```

## 6. Cart Calculation Engine

### Calculation Flow

```
1. Calculate Item Subtotals
   ├── Current Price × Quantity per item
   └── Sum all item subtotals

2. Apply Item-Level Discounts
   ├── BOGO offers
   ├── Volume discounts
   └── Product-specific promotions

3. Calculate Cart Subtotal
   └── Sum of all discounted item totals

4. Apply Cart-Level Promotions
   ├── Percentage discounts
   ├── Fixed amount discounts
   └── Free shipping offers

5. Calculate Tax
   ├── Based on shipping address
   ├── Applied to taxable items only
   └── After discounts applied

6. Calculate Shipping
   ├── Based on weight/dimensions
   ├── Shipping method selected
   └── Apply free shipping if applicable

7. Calculate Final Total
   └── Subtotal + Tax + Shipping - Discounts
```

### Calculation Service

```go
type CalculationService struct {
    taxCalculator      TaxCalculator
    shippingCalculator ShippingCalculator
    promotionEngine    PromotionEngine
    productClient      ProductClient
}

type CartCalculationRequest struct {
    Cart          *Cart
    ShippingZone  string
    TaxZone       string
    ForceRefresh  bool
}

type CartCalculationResponse struct {
    Totals        CartTotals
    ItemBreakdown []ItemCalculation
    TaxBreakdown  []TaxLineItem
    PromotionBreakdown []PromotionLineItem
    Warnings      []string
}
```

## 7. Promotional Code System

### Promotion Types

1. **Fixed Amount Discounts**
   - $5 off orders over $50
   - Flat discount regardless of cart size

2. **Percentage Discounts**
   - 10% off entire order
   - Category-specific percentages

3. **Free Shipping**
   - Remove shipping charges
   - Conditional based on order value

4. **BOGO (Buy One Get One)**
   - Item-specific offers
   - Category-based offers

### Promotion Engine

```go
type PromotionEngine struct {
    promotionClient PromotionClient
    cache          PromotionCache
    validator      PromotionValidator
}

type Promotion struct {
    ID               string           `json:"id"`
    Code             string           `json:"code"`
    Name             string           `json:"name"`
    Type             PromotionType    `json:"type"`
    Value            float64          `json:"value"`
    MinOrderAmount   *float64         `json:"min_order_amount,omitempty"`
    MaxDiscount      *float64         `json:"max_discount,omitempty"`
    EligibleProducts []string         `json:"eligible_products,omitempty"`
    EligibleCategories []string       `json:"eligible_categories,omitempty"`
    StartDate        time.Time        `json:"start_date"`
    EndDate          time.Time        `json:"end_date"`
    UsageLimit       *int             `json:"usage_limit,omitempty"`
    UsageCount       int              `json:"usage_count"`
    StackingRules    StackingRules    `json:"stacking_rules"`
}

type PromotionValidator struct {
    promotionRepo PromotionRepository
}

func (pv *PromotionValidator) ValidatePromotion(ctx context.Context, cart *Cart, promoCode string) (*Promotion, error) {
    // 1. Validate promo code exists and is active
    // 2. Check date validity
    // 3. Check usage limits
    // 4. Validate minimum order requirements
    // 5. Check product/category eligibility
    // 6. Validate stacking rules
}
```

## 8. Mock Database Implementation

### Database Structure

```go
type MockCartDatabase struct {
    carts     map[string]*Cart
    sessions  map[string]*Session
    metadata  DatabaseMetadata
    mutex     sync.RWMutex
    config    MockDBConfig
    persister FilePersister
}

type MockDBConfig struct {
    PersistToFile     bool
    DataDirectory     string
    BackupInterval    time.Duration
    MaxMemoryUsage    int64
    CleanupInterval   time.Duration
}

type FilePersister struct {
    dataDir    string
    backupDir  string
    encryption bool
}
```

### Storage Operations

```go
type CartRepository interface {
    Create(ctx context.Context, cart *Cart) error
    GetByID(ctx context.Context, cartID string) (*Cart, error)
    GetBySessionID(ctx context.Context, sessionID string) (*Cart, error)
    GetByCustomerID(ctx context.Context, customerID string) ([]*Cart, error)
    Update(ctx context.Context, cart *Cart) error
    Delete(ctx context.Context, cartID string) error
    ListExpired(ctx context.Context, before time.Time) ([]*Cart, error)
    CleanupExpired(ctx context.Context, before time.Time) (int, error)
}

type SessionRepository interface {
    Create(ctx context.Context, session *Session) error
    GetByID(ctx context.Context, sessionID string) (*Session, error)
    GetByCustomerID(ctx context.Context, customerID string) ([]*Session, error)
    Update(ctx context.Context, session *Session) error
    Delete(ctx context.Context, sessionID string) error
    CleanupExpired(ctx context.Context, before time.Time) (int, error)
}
```

### Data Persistence

```go
// JSON file structure for persistence
type PersistedData struct {
    Version   string             `json:"version"`
    Timestamp time.Time          `json:"timestamp"`
    Carts     map[string]*Cart   `json:"carts"`
    Sessions  map[string]*Session `json:"sessions"`
    Metadata  DatabaseMetadata   `json:"metadata"`
}

// Backup and recovery
func (db *MockCartDatabase) SaveToFile(ctx context.Context) error {
    // 1. Create backup of current data
    // 2. Write new data to temporary file
    // 3. Atomic move to replace current file
    // 4. Cleanup old backups
}

func (db *MockCartDatabase) LoadFromFile(ctx context.Context) error {
    // 1. Check if data file exists
    // 2. Validate file format and version
    // 3. Load data into memory structures
    // 4. Run data integrity checks
}
```

## 9. OpenTelemetry Instrumentation

### Tracing Points

```go
// Key operations to trace
var (
    TracerName = "griffin-commerce/cart-service"

    // Span names
    SpanCartCreate      = "cart.create"
    SpanCartGet         = "cart.get"
    SpanCartAddItem     = "cart.add_item"
    SpanCartUpdateItem  = "cart.update_item"
    SpanCartRemoveItem  = "cart.remove_item"
    SpanCartCalculate   = "cart.calculate"
    SpanCartMerge       = "cart.merge"
    SpanPromoApply      = "cart.apply_promotion"
    SpanInventoryCheck  = "cart.inventory_check"
    SpanPriceValidation = "cart.price_validation"
)

// HTTP middleware for automatic tracing
func TracingMiddleware(next http.Handler) http.Handler {
    return otelhttp.NewHandler(next, "cart-service")
}

// Manual span creation for business operations
func (cs *CartService) AddItem(ctx context.Context, cartID string, req AddItemRequest) (*AddItemResponse, error) {
    ctx, span := otel.Tracer(TracerName).Start(ctx, SpanCartAddItem)
    defer span.End()

    span.SetAttributes(
        attribute.String("cart.id", cartID),
        attribute.String("product.id", req.ProductID),
        attribute.Int("item.quantity", req.Quantity),
    )

    // Business logic implementation
    result, err := cs.addItemToCart(ctx, cartID, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    span.SetAttributes(
        attribute.Float64("cart.total", result.CartTotals.Total),
        attribute.Int("cart.item_count", len(result.Cart.Items)),
    )

    return result, nil
}
```

### Metrics Collection

```go
var (
    // Cart operation metrics
    CartOperationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cart_operations_total",
            Help: "Total number of cart operations",
        },
        []string{"operation", "status"},
    )

    CartOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "cart_operation_duration_seconds",
            Help: "Duration of cart operations",
            Buckets: prometheus.DefBuckets,
        },
        []string{"operation"},
    )

    // Business metrics
    CartValueDollars = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "cart_value_dollars",
            Help: "Distribution of cart values in dollars",
            Buckets: []float64{10, 25, 50, 100, 250, 500, 1000},
        },
        []string{"customer_type"},
    )

    CartItemsCount = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "cart_items_count",
            Help: "Distribution of items per cart",
            Buckets: []float64{1, 2, 5, 10, 20, 50},
        },
        []string{"customer_type"},
    )

    CartAbandonmentRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cart_abandonment_rate",
            Help: "Cart abandonment rate percentage",
        },
        []string{"time_period"},
    )

    PromotionUsageTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "promotion_usage_total",
            Help: "Total promotion code usage",
        },
        []string{"promo_code", "result"},
    )
)
```

### Custom Attributes and Events

```go
// Custom span attributes for cart operations
const (
    AttributeCartID          = "cart.id"
    AttributeSessionID       = "session.id"
    AttributeCustomerID      = "customer.id"
    AttributeProductID       = "product.id"
    AttributeItemQuantity    = "item.quantity"
    AttributeCartValue       = "cart.value"
    AttributeItemCount       = "cart.item_count"
    AttributePromoCode       = "promotion.code"
    AttributeDiscountAmount  = "promotion.discount_amount"
    AttributeCustomerType    = "customer.type"
)

// Custom events for important business moments
func recordCartConversion(ctx context.Context, cart *Cart) {
    span := trace.SpanFromContext(ctx)
    span.AddEvent("cart.converted", trace.WithAttributes(
        attribute.String(AttributeCartID, cart.ID),
        attribute.Float64(AttributeCartValue, cart.Totals.Total),
        attribute.Int(AttributeItemCount, len(cart.Items)),
        attribute.String(AttributeCustomerType, getCustomerType(cart.CustomerID)),
    ))
}
```

## 10. Testing Strategy

### Unit Testing

```go
// Cart Service Tests
func TestCartService_AddItem(t *testing.T) {
    tests := []struct {
        name           string
        cartID         string
        request        AddItemRequest
        setupMocks     func(*mocks.CartRepository, *mocks.ProductClient)
        expectedError  error
        expectedResult *AddItemResponse
    }{
        {
            name:   "successful_add_item",
            cartID: "cart_123",
            request: AddItemRequest{
                ProductID: "DOG-TOY-001",
                Quantity:  1,
            },
            setupMocks: func(repo *mocks.CartRepository, client *mocks.ProductClient) {
                repo.On("GetByID", mock.Anything, "cart_123").Return(&Cart{
                    ID:     "cart_123",
                    Status: CartStatusActive,
                    Items:  []CartItem{},
                }, nil)

                client.On("GetProduct", mock.Anything, "DOG-TOY-001").Return(&Product{
                    ID:    "DOG-TOY-001",
                    Price: 15.99,
                    InStock: true,
                }, nil)

                repo.On("Update", mock.Anything, mock.AnythingOfType("*Cart")).Return(nil)
            },
            expectedResult: &AddItemResponse{
                Item: CartItem{
                    ProductID: "DOG-TOY-001",
                    Quantity:  1,
                    PriceAtAdd: 15.99,
                },
            },
        },
        {
            name:   "cart_not_found",
            cartID: "cart_404",
            request: AddItemRequest{
                ProductID: "DOG-TOY-001",
                Quantity:  1,
            },
            setupMocks: func(repo *mocks.CartRepository, client *mocks.ProductClient) {
                repo.On("GetByID", mock.Anything, "cart_404").Return(nil, domain.ErrCartNotFound)
            },
            expectedError: domain.ErrCartNotFound,
        },
        {
            name:   "quantity_limit_exceeded",
            cartID: "cart_123",
            request: AddItemRequest{
                ProductID: "DOG-TOY-001",
                Quantity:  100, // Exceeds limit of 99
            },
            setupMocks: func(repo *mocks.CartRepository, client *mocks.ProductClient) {
                repo.On("GetByID", mock.Anything, "cart_123").Return(&Cart{
                    ID:     "cart_123",
                    Status: CartStatusActive,
                    Items:  []CartItem{},
                }, nil)
            },
            expectedError: domain.ErrQuantityLimitExceeded,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks
            mockRepo := &mocks.CartRepository{}
            mockProductClient := &mocks.ProductClient{}
            if tt.setupMocks != nil {
                tt.setupMocks(mockRepo, mockProductClient)
            }

            // Create service
            service := NewCartService(mockRepo, mockProductClient, nil)

            // Execute test
            result, err := service.AddItem(context.Background(), tt.cartID, tt.request)

            // Assert results
            if tt.expectedError != nil {
                assert.Error(t, err)
                assert.Equal(t, tt.expectedError, err)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
                assert.Equal(t, tt.expectedResult.Item.ProductID, result.Item.ProductID)
            }

            // Verify mocks
            mockRepo.AssertExpectations(t)
            mockProductClient.AssertExpectations(t)
        })
    }
}

// Calculation Service Tests
func TestCalculationService_CalculateCartTotals(t *testing.T) {
    tests := []struct {
        name           string
        cart           *Cart
        expectedTotals CartTotals
    }{
        {
            name: "single_item_no_tax_no_shipping",
            cart: &Cart{
                Items: []CartItem{
                    {
                        ProductID:    "DOG-TOY-001",
                        Quantity:     1,
                        CurrentPrice: 15.99,
                    },
                },
            },
            expectedTotals: CartTotals{
                Subtotal: 15.99,
                Tax:      0.00,
                Shipping: 0.00,
                Discount: 0.00,
                Total:    15.99,
                Currency: "USD",
            },
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewCalculationService()
            totals := service.CalculateCartTotals(tt.cart)

            assert.Equal(t, tt.expectedTotals.Subtotal, totals.Subtotal)
            assert.Equal(t, tt.expectedTotals.Total, totals.Total)
        })
    }
}
```

### Integration Testing

```go
// API Integration Tests
func TestCartAPI_CompleteUserJourney(t *testing.T) {
    // Setup test server
    server := setupTestServer()
    defer server.Close()

    client := &http.Client{}

    // Test scenario: Guest user adds items, then logs in and merges cart
    t.Run("guest_to_authenticated_user_flow", func(t *testing.T) {
        // 1. Create guest cart
        createReq := CreateCartRequest{
            SessionID: "guest_session_123",
        }
        cartResp := createCart(t, client, server.URL, createReq)
        guestCartID := cartResp.ID

        // 2. Add items to guest cart
        addItemReq := AddItemRequest{
            ProductID: "DOG-TOY-001",
            Quantity:  2,
        }
        addItem(t, client, server.URL, guestCartID, addItemReq)

        // 3. User logs in, create authenticated cart
        authCreateReq := CreateCartRequest{
            SessionID:  "auth_session_456",
            CustomerID: stringPtr("cust_123"),
        }
        authCartResp := createCart(t, client, server.URL, authCreateReq)
        authCartID := authCartResp.ID

        // 4. Merge guest cart with authenticated cart
        mergeReq := MergeCartRequest{
            SourceCartID: guestCartID,
            TargetCartID: authCartID,
            CustomerID:   "cust_123",
        }
        mergeCart(t, client, server.URL, mergeReq)

        // 5. Verify merged cart contents
        finalCart := getCart(t, client, server.URL, authCartID)
        assert.Len(t, finalCart.Items, 1)
        assert.Equal(t, 2, finalCart.Items[0].Quantity)
        assert.Equal(t, "cust_123", *finalCart.CustomerID)

        // 6. Verify guest cart is cleaned up
        _, err := getCartRaw(client, server.URL, guestCartID)
        assert.Error(t, err) // Should return 404
    })
}

// Performance Tests
func TestCartAPI_PerformanceRequirements(t *testing.T) {
    server := setupTestServer()
    defer server.Close()

    t.Run("cart_operations_under_100ms", func(t *testing.T) {
        client := &http.Client{Timeout: 100 * time.Millisecond}

        // Test create cart performance
        start := time.Now()
        createReq := CreateCartRequest{SessionID: "perf_test_session"}
        cartResp := createCart(t, client, server.URL, createReq)
        createDuration := time.Since(start)

        assert.Less(t, createDuration, 100*time.Millisecond)

        // Test add item performance
        start = time.Now()
        addItemReq := AddItemRequest{ProductID: "DOG-TOY-001", Quantity: 1}
        addItem(t, client, server.URL, cartResp.ID, addItemReq)
        addDuration := time.Since(start)

        assert.Less(t, addDuration, 100*time.Millisecond)
    })
}
```

### Edge Case Testing

```go
func TestCartService_EdgeCases(t *testing.T) {
    t.Run("concurrent_cart_updates", func(t *testing.T) {
        // Test concurrent updates to same cart
        cart := createTestCart()
        service := setupTestService()

        var wg sync.WaitGroup
        errors := make(chan error, 10)

        // Simulate 10 concurrent add item operations
        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(productID string) {
                defer wg.Done()
                _, err := service.AddItem(context.Background(), cart.ID, AddItemRequest{
                    ProductID: productID,
                    Quantity:  1,
                })
                if err != nil {
                    errors <- err
                }
            }(fmt.Sprintf("PRODUCT-%d", i))
        }

        wg.Wait()
        close(errors)

        // Verify no race conditions occurred
        for err := range errors {
            assert.NoError(t, err)
        }

        // Verify final cart state
        finalCart, err := service.GetCart(context.Background(), cart.ID)
        assert.NoError(t, err)
        assert.Len(t, finalCart.Items, 10)
    })

    t.Run("cart_item_limit_enforcement", func(t *testing.T) {
        cart := createTestCart()
        service := setupTestService()

        // Add maximum allowed items (50)
        for i := 0; i < 50; i++ {
            _, err := service.AddItem(context.Background(), cart.ID, AddItemRequest{
                ProductID: fmt.Sprintf("PRODUCT-%d", i),
                Quantity:  1,
            })
            assert.NoError(t, err)
        }

        // Try to add 51st item
        _, err := service.AddItem(context.Background(), cart.ID, AddItemRequest{
            ProductID: "PRODUCT-51",
            Quantity:  1,
        })
        assert.Error(t, err)
        assert.Equal(t, domain.ErrCartItemsLimitExceeded, err)
    })

    t.Run("product_price_change_handling", func(t *testing.T) {
        cart := createTestCart()
        service := setupTestService()

        // Add item at original price
        _, err := service.AddItem(context.Background(), cart.ID, AddItemRequest{
            ProductID: "DOG-TOY-001",
            Quantity:  1,
        })
        assert.NoError(t, err)

        // Simulate price change in product service
        mockProductClient.SetProductPrice("DOG-TOY-001", 19.99) // Was 15.99

        // Recalculate cart totals
        updatedCart, err := service.RecalculateCart(context.Background(), cart.ID)
        assert.NoError(t, err)

        // Verify item shows both prices
        item := updatedCart.Items[0]
        assert.Equal(t, 15.99, item.PriceAtAdd)     // Original price
        assert.Equal(t, 19.99, item.CurrentPrice)   // New price

        // Verify totals use current price
        expectedTotal := 19.99 // New price for calculations
        assert.Equal(t, expectedTotal, updatedCart.Totals.Subtotal)
    })
}
```

### Fault Injection Testing

```go
func TestCartService_FaultInjection(t *testing.T) {
    t.Run("product_service_timeout", func(t *testing.T) {
        // Configure fault injection for product service
        faultConfig := fault.Config{
            ServiceName: "product",
            FailureRate: 1.0, // 100% failure rate
            FailureType: "timeout",
            LatencyMS:   5000, // 5 second timeout
        }

        service := setupTestServiceWithFaults(faultConfig)

        // Attempt to add item when product service is timing out
        ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
        defer cancel()

        _, err := service.AddItem(ctx, "cart_123", AddItemRequest{
            ProductID: "DOG-TOY-001",
            Quantity:  1,
        })

        // Should fail with timeout error
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "timeout")
    })

    t.Run("promotion_service_intermittent_failures", func(t *testing.T) {
        faultConfig := fault.Config{
            ServiceName: "promotion",
            FailureRate: 0.5, // 50% failure rate
            FailureType: "error",
        }

        service := setupTestServiceWithFaults(faultConfig)

        // Try to apply promotion multiple times
        successCount := 0
        errorCount := 0

        for i := 0; i < 100; i++ {
            _, err := service.ApplyPromotion(context.Background(), "cart_123", "SAVE10")
            if err != nil {
                errorCount++
            } else {
                successCount++
            }
        }

        // Should have roughly 50% success rate
        assert.InDelta(t, 50, successCount, 10) // Within 10% of expected
        assert.InDelta(t, 50, errorCount, 10)
    })
}
```

## 11. Integration Points

### Product Catalog Integration

```go
type ProductClient interface {
    GetProduct(ctx context.Context, productID string) (*Product, error)
    GetProducts(ctx context.Context, productIDs []string) ([]*Product, error)
    CheckAvailability(ctx context.Context, productID string, quantity int) (*AvailabilityResponse, error)
    GetCurrentPrice(ctx context.Context, productID string) (*PriceResponse, error)
}

type Product struct {
    ID          string  `json:"id"`
    SKU         string  `json:"sku"`
    Name        string  `json:"name"`
    Price       float64 `json:"price"`
    Currency    string  `json:"currency"`
    InStock     bool    `json:"in_stock"`
    MaxQuantity int     `json:"max_quantity"`
    Weight      float64 `json:"weight"`
    Dimensions  Dimensions `json:"dimensions"`
}

// Mock implementation for POC
type MockProductClient struct {
    products    map[string]*Product
    faultInjector *fault.Injector
}

func (mpc *MockProductClient) GetProduct(ctx context.Context, productID string) (*Product, error) {
    // Apply fault injection
    if shouldFail, err := mpc.faultInjector.ShouldFail(ctx, "get_product", productID); shouldFail {
        return nil, err
    }

    // Simulate network latency
    time.Sleep(mpc.faultInjector.GetLatency("get_product"))

    product, exists := mpc.products[productID]
    if !exists {
        return nil, ErrProductNotFound
    }

    return product, nil
}
```

### Inventory Service Integration

```go
type InventoryClient interface {
    CheckStock(ctx context.Context, productID string, quantity int) (*StockResponse, error)
    ReserveStock(ctx context.Context, reservationReq ReservationRequest) (*ReservationResponse, error)
    ReleaseReservation(ctx context.Context, reservationID string) error
}

type ReservationRequest struct {
    ProductID    string        `json:"product_id"`
    Quantity     int           `json:"quantity"`
    CartID       string        `json:"cart_id"`
    ReserveTTL   time.Duration `json:"reserve_ttl"`
}

type ReservationResponse struct {
    ReservationID string    `json:"reservation_id"`
    ProductID     string    `json:"product_id"`
    Quantity      int       `json:"quantity"`
    ExpiresAt     time.Time `json:"expires_at"`
}
```

### Promotion Service Integration

```go
type PromotionClient interface {
    ValidatePromoCode(ctx context.Context, promoCode string, cart *Cart) (*PromotionValidationResponse, error)
    ApplyPromotion(ctx context.Context, promoCode string, cart *Cart) (*PromotionApplicationResponse, error)
    GetActivePromotions(ctx context.Context, customerID *string) ([]*Promotion, error)
}

type PromotionValidationResponse struct {
    Valid         bool      `json:"valid"`
    Promotion     *Promotion `json:"promotion,omitempty"`
    ErrorCode     string    `json:"error_code,omitempty"`
    ErrorMessage  string    `json:"error_message,omitempty"`
}
```

## 12. Configuration

### Service Configuration

```yaml
# config/cart-service.yaml
app:
  name: "cart-service"
  environment: "poc"
  port: 8082
  log_level: "debug"
  shutdown_timeout: "30s"

cart:
  max_items_per_cart: 50
  max_quantity_per_item: 99
  session_timeout_minutes: 30
  cart_expiry_days: 30
  cleanup_interval_minutes: 60
  reservation_ttl_minutes: 15

database:
  mock_mode: true
  persist_to_file: true
  data_directory: "./data/cart"
  backup_interval_minutes: 15
  max_memory_usage_mb: 512

telemetry:
  enabled: true
  service_name: "cart-service"
  otlp_endpoint: "localhost:4317"
  sampling_rate: 1.0
  metrics_endpoint: "/metrics"

external_services:
  product_service:
    base_url: "http://localhost:8080"
    timeout: "5s"
    retry_attempts: 3
    retry_delay: "1s"

  inventory_service:
    base_url: "http://localhost:8081"
    timeout: "3s"
    retry_attempts: 2
    retry_delay: "500ms"

  promotion_service:
    base_url: "http://localhost:8084"
    timeout: "2s"
    retry_attempts: 2
    retry_delay: "300ms"

fault_injection:
  enabled: true
  product_service:
    failure_rate: 0.0
    latency_ms: 50
  inventory_service:
    failure_rate: 0.05
    latency_ms: 30
  promotion_service:
    failure_rate: 0.1
    latency_ms: 100

rate_limiting:
  enabled: true
  requests_per_minute: 1000
  burst_size: 100

security:
  cors_enabled: true
  cors_origins: ["http://localhost:3000"]
  max_request_size_mb: 10
```

## 13. Implementation Order

### Phase 1: Core Infrastructure (Week 1)
1. **Project Setup**
   - Initialize Go module and directory structure
   - Setup configuration management (Viper)
   - Implement basic logging with structured fields
   - Create mock database framework

2. **Domain Models**
   - Define Cart, CartItem, Session domain models
   - Implement domain errors and validation
   - Create repository interfaces

3. **Mock Database**
   - Implement in-memory storage with thread safety
   - Add JSON file persistence capability
   - Create basic CRUD operations

### Phase 2: Core Cart Operations (Week 2)
1. **Cart Service**
   - Implement CreateCart, GetCart operations
   - Add AddItem, UpdateItem, RemoveItem functionality
   - Implement basic cart validation rules

2. **Session Management**
   - Create session service with guest/authenticated handling
   - Implement session expiration and cleanup
   - Add cart-session linking logic

3. **API Layer**
   - Setup HTTP server with middleware
   - Implement cart CRUD endpoints
   - Add request validation and error handling

### Phase 3: Advanced Features (Week 3)
1. **Calculation Engine**
   - Implement cart totals calculation
   - Add tax and shipping calculation placeholders
   - Create price validation against product service

2. **Promotion System**
   - Implement promotion validation
   - Add discount calculation logic
   - Create promotion application workflow

3. **Cart Merging**
   - Implement guest-to-authenticated cart merging
   - Add duplicate item handling
   - Create merge conflict resolution

### Phase 4: Integration & Observability (Week 4)
1. **Service Integration**
   - Implement product service client
   - Add inventory service integration
   - Create promotion service client

2. **OpenTelemetry**
   - Add tracing to all operations
   - Implement custom metrics collection
   - Create structured logging throughout

3. **Fault Injection**
   - Implement fault injection framework
   - Add configurable failure scenarios
   - Create fault testing endpoints

### Phase 5: Testing & Performance (Week 5)
1. **Unit Testing**
   - Write comprehensive unit tests for all services
   - Add test coverage reporting
   - Create test data factories

2. **Integration Testing**
   - Implement API integration tests
   - Add performance requirement tests
   - Create fault injection test scenarios

3. **Load Testing**
   - Setup concurrent cart operation tests
   - Add memory usage monitoring
   - Create cleanup and maintenance procedures

## 14. Success Criteria

### Functional Requirements
- ✅ Support guest and authenticated user carts
- ✅ Handle cart operations (CRUD) under 100ms
- ✅ Merge guest carts with authenticated user carts
- ✅ Apply promotional codes and calculate discounts
- ✅ Enforce business rules (item limits, quantity limits)
- ✅ Maintain cart persistence across sessions

### Performance Requirements
- ✅ Response time < 100ms for all operations
- ✅ Support 10,000+ concurrent active carts
- ✅ Memory usage < 512MB for mock database
- ✅ Handle concurrent cart updates without race conditions

### Observability Requirements
- ✅ Full OpenTelemetry tracing for all operations
- ✅ Business metrics collection (cart value, abandonment rate)
- ✅ Structured logging with correlation IDs
- ✅ Health and readiness endpoints

### Testing Requirements
- ✅ Unit test coverage > 90%
- ✅ Integration tests for all API endpoints
- ✅ Edge case testing (concurrent updates, limits)
- ✅ Fault injection testing scenarios

### Integration Requirements
- ✅ Product service integration for pricing and availability
- ✅ Inventory service integration for stock management
- ✅ Promotion service integration for discount codes
- ✅ Graceful handling of external service failures

This comprehensive system design provides a complete blueprint for implementing the Shopping Cart Service with all required features, proper architecture, extensive testing, and full observability. The design prioritizes simplicity while ensuring production-ready quality and maintainability.