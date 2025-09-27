# Product Catalog Service Design

## Overview
The Product Catalog Service is the foundational data service for the Griffin Commerce demo e-commerce platform. It serves as the single source of truth for all product information, managing inventory, pricing, and product metadata for the dog product store. The service is designed as a POC with mock implementations and full OpenTelemetry instrumentation.

## System Architecture

### Service Overview
- **Package**: `package catalog`
- **Port**: 8080 (base service)
- **Storage**: In-memory mock database with optional JSON persistence
- **Data Loading**: YAML-driven initial product data
- **External Dependencies**: None (all mocked for POC)

### Core Components

#### 1. Product Manager
**Responsibility**: Core business logic for product operations
**Location**: `/catalog/manager/product_manager.go`

Functions:
- Product CRUD operations
- Inventory management
- Price calculations
- Category validation
- Search and filtering logic
- Business rule enforcement

#### 2. Mock Database Layer
**Responsibility**: Data persistence and retrieval
**Location**: `/catalog/storage/mock_db.go`

Features:
- Thread-safe in-memory storage using sync.RWMutex
- Generic CRUD interface
- JSON persistence for data recovery
- Bulk operations support
- Query filtering and sorting capabilities

#### 3. API Handler Layer
**Responsibility**: HTTP request handling and response formatting
**Location**: `/catalog/handlers/`

Handlers:
- Product CRUD handlers
- Search and filter handlers
- Category handlers
- Inventory handlers
- Health check handlers

#### 4. Search Engine
**Responsibility**: Product search and filtering functionality
**Location**: `/catalog/search/search_engine.go`

Features:
- Full-text search on name, description, SKU
- Multi-field filtering
- Sorting capabilities
- Pagination support
- Cache-optimized queries

#### 5. Inventory Manager
**Responsibility**: Stock tracking and availability management
**Location**: `/catalog/inventory/manager.go`

Features:
- Stock level tracking
- Availability calculations
- Reservation management (for cart integration)
- Low stock alerts
- Bulk inventory updates

#### 6. Cache Layer
**Responsibility**: Performance optimization for frequently accessed data
**Location**: `/catalog/cache/product_cache.go`

Features:
- In-memory LRU cache
- TTL support (configurable, default 300 seconds)
- Cache warming from YAML data
- Cache invalidation on updates
- Performance metrics

## Data Models

### Product Entity
```go
type Product struct {
    ID           string                 `json:"id" yaml:"id"`
    SKU          string                 `json:"sku" yaml:"sku"`
    Name         string                 `json:"name" yaml:"name"`
    Description  string                 `json:"description" yaml:"description"`
    Category     string                 `json:"category" yaml:"category"`
    Subcategory  string                 `json:"subcategory,omitempty" yaml:"subcategory,omitempty"`
    Price        float64                `json:"price" yaml:"price"`
    Currency     string                 `json:"currency" yaml:"currency"`
    Weight       Weight                 `json:"weight" yaml:"weight"`
    Dimensions   Dimensions             `json:"dimensions" yaml:"dimensions"`
    Images       []ProductImage         `json:"images" yaml:"images"`
    Inventory    int                    `json:"inventory" yaml:"inventory"`
    Tags         []string               `json:"tags" yaml:"tags"`
    Attributes   map[string]interface{} `json:"attributes,omitempty" yaml:"attributes,omitempty"`
    Variants     []ProductVariant       `json:"variants,omitempty" yaml:"variants,omitempty"`
    CreatedAt    time.Time              `json:"created_at"`
    UpdatedAt    time.Time              `json:"updated_at"`
    IsActive     bool                   `json:"is_active"`
}

type Weight struct {
    Value float64 `json:"value" yaml:"value"`
    Unit  string  `json:"unit" yaml:"unit"` // "oz", "lb", "g", "kg"
}

type Dimensions struct {
    Length float64 `json:"length" yaml:"length"`
    Width  float64 `json:"width" yaml:"width"`
    Height float64 `json:"height" yaml:"height"`
    Unit   string  `json:"unit" yaml:"unit"` // "in", "cm"
}

type ProductImage struct {
    URL      string `json:"url" yaml:"url"`
    Alt      string `json:"alt" yaml:"alt"`
    Type     string `json:"type,omitempty" yaml:"type,omitempty"` // "primary", "gallery", "variant"
    Position int    `json:"position,omitempty" yaml:"position,omitempty"`
}

type ProductVariant struct {
    ID          string            `json:"id" yaml:"id"`
    Name        string            `json:"name" yaml:"name"`
    Attributes  map[string]string `json:"attributes" yaml:"attributes"` // size, color, flavor
    Price       float64           `json:"price" yaml:"price"`
    Inventory   int               `json:"inventory" yaml:"inventory"`
    SKU         string            `json:"sku" yaml:"sku"`
    IsAvailable bool              `json:"is_available"`
}
```

### Category Entity
```go
type Category struct {
    ID          string   `json:"id" yaml:"id"`
    Name        string   `json:"name" yaml:"name"`
    Description string   `json:"description" yaml:"description"`
    ParentID    string   `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
    Subcategories []string `json:"subcategories,omitempty" yaml:"subcategories,omitempty"`
    IsActive    bool     `json:"is_active"`
    SortOrder   int      `json:"sort_order" yaml:"sort_order"`
}
```

### Inventory Entity
```go
type InventoryItem struct {
    ProductID     string    `json:"product_id"`
    VariantID     string    `json:"variant_id,omitempty"`
    StockCount    int       `json:"stock_count"`
    ReservedCount int       `json:"reserved_count"`
    AvailableCount int      `json:"available_count"`
    LowStockThreshold int   `json:"low_stock_threshold"`
    LastUpdated   time.Time `json:"last_updated"`
}
```

### Search Models
```go
type SearchRequest struct {
    Query       string            `json:"query,omitempty"`
    Category    string            `json:"category,omitempty"`
    MinPrice    *float64          `json:"min_price,omitempty"`
    MaxPrice    *float64          `json:"max_price,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    InStock     *bool             `json:"in_stock,omitempty"`
    Attributes  map[string]string `json:"attributes,omitempty"`
    SortBy      string            `json:"sort_by,omitempty"` // "price", "name", "popularity", "newest"
    SortOrder   string            `json:"sort_order,omitempty"` // "asc", "desc"
    Page        int               `json:"page,omitempty"`
    PageSize    int               `json:"page_size,omitempty"`
}

type SearchResponse struct {
    Products    []Product `json:"products"`
    TotalCount  int       `json:"total_count"`
    Page        int       `json:"page"`
    PageSize    int       `json:"page_size"`
    TotalPages  int       `json:"total_pages"`
    Facets      Facets    `json:"facets"`
}

type Facets struct {
    Categories map[string]int `json:"categories"`
    PriceRanges []PriceRange  `json:"price_ranges"`
    Tags       map[string]int `json:"tags"`
}

type PriceRange struct {
    Min   float64 `json:"min"`
    Max   float64 `json:"max"`
    Count int     `json:"count"`
}
```

## API Design

### Product Operations

#### GET /api/catalog/products
**Description**: List products with optional filtering and pagination
**Query Parameters**:
- `category` (string): Filter by category
- `min_price` (float): Minimum price filter
- `max_price` (float): Maximum price filter
- `tags` ([]string): Filter by tags
- `in_stock` (bool): Filter by availability
- `page` (int): Page number (default: 1)
- `page_size` (int): Items per page (default: 20, max: 100)
- `sort_by` (string): Sort field (price, name, popularity, newest)
- `sort_order` (string): Sort direction (asc, desc)

**Response**:
```json
{
  "products": [...],
  "total_count": 150,
  "page": 1,
  "page_size": 20,
  "total_pages": 8,
  "facets": {
    "categories": {"toys": 45, "treats": 30, "food": 25},
    "price_ranges": [
      {"min": 0, "max": 10, "count": 35},
      {"min": 10, "max": 25, "count": 45}
    ],
    "tags": {"large-dogs": 25, "puppies": 15}
  }
}
```

#### GET /api/catalog/products/{id}
**Description**: Get product by ID
**Response**: Product entity with full details

#### POST /api/catalog/products
**Description**: Create new product
**Request Body**: Product entity (without ID, timestamps)
**Response**: Created product with generated ID and timestamps

#### PUT /api/catalog/products/{id}
**Description**: Update product
**Request Body**: Product entity
**Response**: Updated product entity

#### DELETE /api/catalog/products/{id}
**Description**: Delete product (soft delete - sets is_active to false)
**Response**: 204 No Content

### Search Operations

#### GET /api/catalog/search
**Description**: Search products with full-text search
**Query Parameters**:
- `q` (string): Search query
- All filtering parameters from product listing
**Response**: SearchResponse with matching products and facets

#### GET /api/catalog/search/suggestions
**Description**: Get search suggestions/autocomplete
**Query Parameters**:
- `q` (string): Partial search query
- `limit` (int): Maximum suggestions (default: 10)
**Response**:
```json
{
  "suggestions": [
    {"text": "Premium Rope Toy", "type": "product"},
    {"text": "rope toys", "type": "category"}
  ]
}
```

### Category Operations

#### GET /api/catalog/categories
**Description**: List all categories
**Response**: Array of Category entities

#### GET /api/catalog/categories/{id}
**Description**: Get category by ID
**Response**: Category entity

#### GET /api/catalog/categories/{id}/products
**Description**: Get products in category
**Query Parameters**: Same as product listing
**Response**: SearchResponse with products in category

### Inventory Operations

#### GET /api/catalog/inventory/{productId}
**Description**: Get inventory status for product
**Response**: InventoryItem entity

#### PUT /api/catalog/inventory/{productId}
**Description**: Update inventory levels
**Request Body**:
```json
{
  "stock_count": 100,
  "low_stock_threshold": 10
}
```
**Response**: Updated InventoryItem entity

#### POST /api/catalog/inventory/bulk-update
**Description**: Bulk inventory update
**Request Body**:
```json
{
  "updates": [
    {"product_id": "DOG-TOY-001", "stock_count": 150},
    {"product_id": "DOG-TREAT-001", "stock_count": 75}
  ]
}
```
**Response**: Array of updated InventoryItem entities

#### POST /api/catalog/inventory/reserve
**Description**: Reserve inventory for cart (15-minute hold)
**Request Body**:
```json
{
  "items": [
    {"product_id": "DOG-TOY-001", "quantity": 2},
    {"product_id": "DOG-TREAT-001", "quantity": 1}
  ],
  "reservation_id": "cart-uuid-123"
}
```
**Response**: Reservation confirmation with expiry time

#### DELETE /api/catalog/inventory/reserve/{reservationId}
**Description**: Release inventory reservation
**Response**: 204 No Content

### Bulk Operations

#### POST /api/catalog/bulk/import
**Description**: Bulk import products from YAML/JSON
**Request Body**: Multipart form with file upload
**Response**: Import summary with success/error counts

#### GET /api/catalog/bulk/export
**Description**: Export all products as YAML/JSON
**Query Parameters**:
- `format` (string): "yaml" or "json" (default: yaml)
**Response**: Product data in requested format

### Health and Debug Endpoints

#### GET /health
**Description**: Service health check
**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "dependencies": {
    "database": "healthy",
    "cache": "healthy"
  }
}
```

#### GET /ready
**Description**: Readiness probe
**Response**: 200 OK when service is ready to accept requests

#### GET /debug/config
**Description**: Current configuration (sanitized)
**Response**: Configuration object with sensitive values masked

#### GET /debug/cache/stats
**Description**: Cache performance statistics
**Response**:
```json
{
  "hit_rate": 0.85,
  "total_requests": 1500,
  "cache_hits": 1275,
  "cache_misses": 225,
  "evictions": 10,
  "size": 500
}
```

## Search and Filtering Implementation

### Search Strategy
1. **Primary Search**: Full-text search on name and description using Go's built-in string matching
2. **Secondary Filters**: Exact matches on category, tags, price ranges
3. **Ranking**: Score-based ranking considering:
   - Exact matches in name (highest score)
   - Partial matches in name
   - Matches in description
   - Tag matches
   - Popularity (simulated based on inventory turnover)

### Search Index Structure
```go
type SearchIndex struct {
    Products      map[string]*Product
    NameIndex     map[string][]string          // word -> product IDs
    CategoryIndex map[string][]string          // category -> product IDs
    TagIndex      map[string][]string          // tag -> product IDs
    PriceIndex    *PriceRangeIndex
    mutex         sync.RWMutex
}

type PriceRangeIndex struct {
    Ranges []PriceRange
    Index  map[int][]string // range index -> product IDs
}
```

### Search Algorithm
1. **Query Processing**: Tokenize search query, normalize case
2. **Primary Matching**: Find products matching any query tokens
3. **Filtering**: Apply category, price, tag, and availability filters
4. **Scoring**: Calculate relevance scores for each result
5. **Sorting**: Apply requested sort order or relevance ranking
6. **Pagination**: Return requested page of results
7. **Facet Generation**: Calculate facets based on filtered result set

### Performance Optimizations
- **Index Maintenance**: Rebuild indexes on product updates
- **Cache Integration**: Cache popular search results
- **Concurrent Access**: Use RWMutex for thread-safe operations
- **Memory Efficiency**: Use string slices for index storage

## Inventory Management

### Stock Tracking
- **Available Stock**: Total inventory minus reserved items
- **Reserved Stock**: Items held for active carts (15-minute TTL)
- **Low Stock Alerts**: Configurable thresholds per product
- **Stock History**: Track inventory changes for analytics

### Reservation System
```go
type ReservationManager struct {
    reservations map[string]*Reservation
    mutex        sync.RWMutex
    ttl          time.Duration
}

type Reservation struct {
    ID        string
    Items     []ReservationItem
    ExpiresAt time.Time
    CreatedAt time.Time
}

type ReservationItem struct {
    ProductID string
    Quantity  int
}
```

### Business Rules
- **Reservation TTL**: 15 minutes default (configurable)
- **Automatic Cleanup**: Background goroutine removes expired reservations
- **Overselling Prevention**: Check available stock before reservation
- **Batch Operations**: Support bulk inventory updates
- **Audit Trail**: Log all inventory changes for tracking

## Caching Strategy

### Cache Layers
1. **Product Cache**: Frequently accessed products (TTL: 5 minutes)
2. **Search Cache**: Popular search queries (TTL: 2 minutes)
3. **Category Cache**: Category hierarchies (TTL: 1 hour)
4. **Inventory Cache**: Stock levels (TTL: 30 seconds)

### Cache Implementation
```go
type ProductCache struct {
    cache     map[string]*CacheEntry
    mutex     sync.RWMutex
    ttl       time.Duration
    maxSize   int
    lruList   *list.List // For LRU eviction
}

type CacheEntry struct {
    Value     interface{}
    ExpiresAt time.Time
    AccessCount int
    LastAccess  time.Time
    lruElement  *list.Element
}
```

### Cache Strategies
- **Cache-Aside**: Load data into cache on miss
- **Write-Through**: Update cache on data modification
- **Cache Warming**: Pre-load popular products on startup
- **Smart Eviction**: LRU with access frequency consideration

### Cache Invalidation
- **Product Updates**: Invalidate specific product cache entries
- **Inventory Changes**: Invalidate inventory and related search caches
- **Bulk Operations**: Clear related cache segments
- **TTL-Based**: Automatic expiration for data freshness

## YAML Data Loading and Persistence

### Initial Data Loading
**File**: `/data/products.yaml`
**Load Process**:
1. Parse YAML file on service startup
2. Validate product data against schema
3. Insert products into mock database
4. Build search indexes
5. Warm cache with popular items

### YAML Schema Validation
```go
type YAMLValidator struct {
    requiredFields []string
    categoryWhitelist []string
    currencyWhitelist []string
}

func (v *YAMLValidator) ValidateProduct(product *Product) error {
    // Validate required fields
    // Check category against whitelist
    // Validate price and currency
    // Ensure SKU uniqueness
    // Validate weight and dimensions
}
```

### JSON Persistence (Optional)
**Directory**: `/data/persistence/`
**Files**:
- `products.json`: Product data
- `inventory.json`: Inventory levels
- `categories.json`: Category data
- `metadata.json`: Service metadata

**Persistence Strategy**:
- **Periodic Saves**: Every 5 minutes (configurable)
- **Shutdown Save**: Persist data on graceful shutdown
- **Recovery**: Load persisted data on startup if available
- **Backup**: Rotate JSON files to prevent data loss

## OpenTelemetry Instrumentation

### Automatic Instrumentation
- **HTTP Middleware**: Request/response tracing for all endpoints
- **Database Operations**: Span creation for all CRUD operations
- **Cache Operations**: Trace cache hits/misses and performance
- **Search Operations**: Detailed search query and result tracing

### Custom Spans
```go
// Product operations
span := tracer.Start(ctx, "catalog.product.create")
span.SetAttributes(
    attribute.String("product.id", product.ID),
    attribute.String("product.sku", product.SKU),
    attribute.String("product.category", product.Category),
)

// Search operations
span := tracer.Start(ctx, "catalog.search.execute")
span.SetAttributes(
    attribute.String("search.query", request.Query),
    attribute.String("search.category", request.Category),
    attribute.Int("search.results", len(results)),
    attribute.Int64("search.duration_ms", duration.Milliseconds()),
)

// Inventory operations
span := tracer.Start(ctx, "catalog.inventory.reserve")
span.SetAttributes(
    attribute.Int("inventory.items_count", len(items)),
    attribute.String("inventory.reservation_id", reservationID),
)
```

### Business Metrics
```go
// Counters
ProductViewsCounter := metric.NewInt64Counter("catalog_product_views_total")
SearchRequestsCounter := metric.NewInt64Counter("catalog_search_requests_total")
InventoryReservationsCounter := metric.NewInt64Counter("catalog_inventory_reservations_total")

// Histograms
SearchLatencyHistogram := metric.NewFloat64Histogram("catalog_search_duration_seconds")
DatabaseOperationHistogram := metric.NewFloat64Histogram("catalog_db_operation_duration_seconds")

// Gauges
ActiveProductsGauge := metric.NewInt64ObservableGauge("catalog_active_products")
LowStockProductsGauge := metric.NewInt64ObservableGauge("catalog_low_stock_products")
CacheHitRateGauge := metric.NewFloat64ObservableGauge("catalog_cache_hit_rate")
```

### Error Tracking
- **Span Status**: Set error status on span for failed operations
- **Error Attributes**: Include error details and context
- **Error Aggregation**: Count errors by type and endpoint
- **Alert Thresholds**: Define SLOs for error rates

## Integration Patterns with Other Services

### Shopping Cart Service Integration
**Purpose**: Provide product details and pricing for cart operations

**API Contract**:
```go
// Endpoint: GET /api/catalog/products/batch
// Request: {"product_ids": ["DOG-TOY-001", "DOG-TREAT-001"]}
// Response: {"products": [...], "missing_ids": []}

// Endpoint: POST /api/catalog/inventory/reserve
// Purpose: Reserve inventory when items added to cart
// TTL: 15 minutes (configurable)
```

**Integration Points**:
- Cart validates product existence and pricing
- Inventory reservation prevents overselling
- Real-time price updates to cart service
- Product availability status for cart display

### Image Service Integration
**Purpose**: Link product images with catalog data

**API Contract**:
```go
// Product contains image references
type ProductImage struct {
    URL    string `json:"url"`    // Points to Image Service
    Alt    string `json:"alt"`
    Type   string `json:"type"`   // primary, gallery, variant
}

// Image Service provides:
// GET /api/images/product/{productId} - Get all product images
// POST /api/images/upload - Upload new product images
```

**Integration Points**:
- Product creation triggers image association
- Image URLs stored in product catalog
- Image metadata synchronized between services
- CDN URLs for optimized image delivery

### Recommendations Service Integration
**Purpose**: Provide product metadata for recommendation algorithms

**API Contract**:
```go
// Endpoint: GET /api/catalog/products/{id}/metadata
// Response: Product with analytics-friendly metadata

// Endpoint: GET /api/catalog/analytics/popular
// Response: Top-selling products by category

// Endpoint: POST /api/catalog/analytics/interactions
// Purpose: Record product interactions for recommendations
```

**Integration Points**:
- Product attributes feed recommendation engine
- Purchase history data shared for collaborative filtering
- Category relationships for content-based recommendations
- Product popularity metrics for trending calculations

### Payment/Order Integration
**Purpose**: Validate product information during checkout

**API Contract**:
```go
// Endpoint: POST /api/catalog/validate/checkout
// Request: Cart items with quantities
// Response: Validated items with current prices and availability

// Endpoint: POST /api/catalog/inventory/commit
// Purpose: Confirm inventory deduction after successful payment
```

**Integration Points**:
- Price validation during payment processing
- Inventory commitment after successful payment
- Product data for order confirmation
- SKU verification for fulfillment

## File Structure

```
/catalog/
├── main.go                          # Service entry point
├── config/
│   └── config.go                    # Configuration management
├── handlers/
│   ├── products.go                  # Product CRUD handlers
│   ├── search.go                    # Search and filter handlers
│   ├── categories.go                # Category handlers
│   ├── inventory.go                 # Inventory handlers
│   └── health.go                    # Health check handlers
├── manager/
│   ├── product_manager.go           # Core product business logic
│   ├── inventory_manager.go         # Inventory management logic
│   └── category_manager.go          # Category management logic
├── storage/
│   ├── interface.go                 # Storage interface definitions
│   ├── mock_db.go                   # Mock database implementation
│   └── persistence.go               # JSON persistence layer
├── search/
│   ├── search_engine.go             # Search implementation
│   ├── indexer.go                   # Search index management
│   └── filters.go                   # Filtering logic
├── cache/
│   ├── product_cache.go             # Product caching layer
│   ├── search_cache.go              # Search result caching
│   └── inventory_cache.go           # Inventory caching
├── models/
│   ├── product.go                   # Product data models
│   ├── category.go                  # Category data models
│   ├── inventory.go                 # Inventory data models
│   └── search.go                    # Search request/response models
├── middleware/
│   ├── logging.go                   # Request logging middleware
│   ├── metrics.go                   # Metrics collection middleware
│   ├── validation.go                # Request validation middleware
│   └── cors.go                      # CORS handling middleware
├── telemetry/
│   ├── metrics.go                   # Custom business metrics
│   ├── tracing.go                   # OpenTelemetry tracing setup
│   └── monitoring.go                # Health and monitoring
├── utils/
│   ├── validation.go                # Data validation utilities
│   ├── pagination.go                # Pagination helpers
│   └── currency.go                  # Currency handling utilities
└── testdata/
    ├── products.yaml                # Sample product data
    ├── categories.yaml              # Sample category data
    └── test_scenarios.yaml          # Test data scenarios
```

## Testing Strategy

### Unit Testing

#### Product Manager Tests
```go
func TestProductManager_CreateProduct(t *testing.T) {
    // Test product creation with valid data
    // Test validation of required fields
    // Test SKU uniqueness enforcement
    // Test price validation
}

func TestProductManager_UpdateInventory(t *testing.T) {
    // Test inventory updates
    // Test concurrent inventory updates
    // Test negative inventory prevention
    // Test reservation system
}
```

#### Search Engine Tests
```go
func TestSearchEngine_ProductSearch(t *testing.T) {
    // Test full-text search functionality
    // Test search ranking accuracy
    // Test filter combinations
    // Test pagination
    // Test facet generation
}

func TestSearchEngine_Performance(t *testing.T) {
    // Test search latency with large datasets
    // Test concurrent search operations
    // Test index rebuild performance
}
```

#### Cache Tests
```go
func TestProductCache_CacheOperations(t *testing.T) {
    // Test cache hit/miss scenarios
    // Test TTL expiration
    // Test LRU eviction
    // Test concurrent access
    // Test cache invalidation
}
```

### Integration Testing

#### API Endpoint Tests
```go
func TestProductAPI_CRUD(t *testing.T) {
    // Test complete product CRUD lifecycle
    // Test API request/response format
    // Test error handling
    // Test authentication/authorization
}

func TestSearchAPI_Integration(t *testing.T) {
    // Test search API with realistic data
    // Test filter combinations
    // Test sorting and pagination
    // Test facet accuracy
}
```

#### Service Integration Tests
```go
func TestCartIntegration(t *testing.T) {
    // Test inventory reservation flow
    // Test price validation
    // Test product availability checks
}

func TestRecommendationIntegration(t *testing.T) {
    // Test product metadata sharing
    // Test analytics data exchange
}
```

### Performance Testing

#### Load Testing Scenarios
```go
func BenchmarkProductSearch(b *testing.B) {
    // Test search performance under load
    // Measure query response times
    // Test concurrent search operations
}

func BenchmarkInventoryOperations(b *testing.B) {
    // Test inventory update performance
    // Test reservation system under load
    // Measure database operation latency
}
```

#### Memory and Resource Testing
- **Memory Usage**: Monitor memory consumption with large product catalogs
- **CPU Usage**: Profile search and filtering operations
- **Cache Performance**: Measure cache hit rates and eviction patterns
- **Goroutine Safety**: Test concurrent operations for race conditions

### Test Data Scenarios

#### Normal Operations
- **Standard Catalog**: 1,000 products across 10 categories
- **Large Catalog**: 50,000 products for performance testing
- **Diverse Products**: Various price ranges, sizes, and attributes

#### Edge Cases
- **Empty Catalog**: Test behavior with no products
- **Single Product**: Test with minimal data
- **Large Product**: Test with maximum field sizes
- **Invalid Data**: Test validation and error handling

#### Stress Testing
- **High Traffic**: Simulate 10,000 concurrent requests
- **Memory Pressure**: Test with constrained memory limits
- **Slow Operations**: Test with artificial latency injection
- **Failover Scenarios**: Test service recovery mechanisms

## Configuration

### Environment Variables
```yaml
# Service Configuration
CATALOG_PORT=8080
CATALOG_LOG_LEVEL=debug
CATALOG_ENV=poc

# Data Configuration
CATALOG_DATA_DIR=./data
CATALOG_PERSIST_DATA=true
CATALOG_YAML_FILE=./data/products.yaml

# Cache Configuration
CATALOG_CACHE_TTL_SECONDS=300
CATALOG_CACHE_MAX_SIZE=10000
CATALOG_CACHE_WARMUP=true

# Search Configuration
CATALOG_SEARCH_MAX_RESULTS=1000
CATALOG_SEARCH_DEFAULT_PAGE_SIZE=20

# Inventory Configuration
CATALOG_RESERVATION_TTL_MINUTES=15
CATALOG_LOW_STOCK_THRESHOLD=10

# OpenTelemetry Configuration
OTEL_SERVICE_NAME=catalog-service
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_TRACES_SAMPLER=traceidratio
OTEL_TRACES_SAMPLER_ARG=1.0

# Mock Configuration
CATALOG_MOCK_DB_LATENCY_MS=10
CATALOG_FAULT_INJECTION_ENABLED=true
```

### YAML Configuration File
```yaml
# config.yaml
service:
  name: "Product Catalog Service"
  version: "1.0.0"
  port: 8080
  environment: "poc"

database:
  type: "mock"
  persist: true
  data_directory: "./data"
  initial_data_file: "./data/products.yaml"

cache:
  enabled: true
  ttl_seconds: 300
  max_size: 10000
  warmup_enabled: true

search:
  max_results: 1000
  default_page_size: 20
  facets_enabled: true

inventory:
  reservation_ttl_minutes: 15
  low_stock_threshold: 10
  auto_cleanup_enabled: true

telemetry:
  enabled: true
  metrics_enabled: true
  tracing_enabled: true
  logging_level: "debug"

fault_injection:
  enabled: true
  database_failures:
    rate: 0.01
    types: ["timeout", "connection_error"]
  cache_failures:
    rate: 0.005
    types: ["eviction", "timeout"]
```

## Implementation Notes

### POC Simplifications
- **No External Database**: Use in-memory storage with optional JSON persistence
- **Simplified Search**: Basic string matching instead of Elasticsearch
- **Mock Dependencies**: All external services are mocked
- **No Authentication**: Focus on core functionality
- **Basic Validation**: Essential validation only

### Production Readiness Considerations
- **Database Migration**: Plan for PostgreSQL/MongoDB integration
- **Search Engine**: Consider Elasticsearch for advanced search
- **Authentication**: Add JWT/OAuth2 support
- **Rate Limiting**: Implement API rate limiting
- **Caching**: Consider Redis for distributed caching
- **Monitoring**: Enhanced metrics and alerting

### Performance Expectations
- **Product Retrieval**: < 50ms for single product
- **Search Operations**: < 200ms for complex queries
- **Inventory Updates**: < 100ms for single product
- **Bulk Operations**: < 5 seconds for 1000 products
- **Cache Performance**: 95%+ hit rate for popular products

This design provides a comprehensive foundation for the Product Catalog Service that serves as the cornerstone of the Griffin Commerce platform while maintaining POC simplicity and full observability through OpenTelemetry instrumentation.