# Simplified Common Service Design

## Overview

The Common Service provides basic shared utilities for all other services in the Griffin Commerce demo. This is an extremely simplified design focused on essential functionality only for a 2-week POC.

## System Architecture

### Design Philosophy
- EXTREME SIMPLICITY - no unnecessary abstractions
- Single responsibility per file
- No external dependencies beyond Go stdlib and OpenTelemetry
- Flat package structure
- No interfaces unless absolutely required

### Component Architecture

```
common/
├── config.go      # YAML configuration loading
├── database.go    # Simple in-memory map with mutex
├── middleware.go  # HTTP middleware (logging, tracing, correlation)
├── errors.go      # Basic error types
├── telemetry.go   # OpenTelemetry setup (HTTP only)
└── models.go      # Shared data structures
```

## Detailed Component Specifications

### 1. Configuration Component (`config.go`)

**Purpose**: Load configuration from single YAML file at startup

**Implementation Requirements**:
```go
type Config struct {
    Services map[string]ServiceConfig `yaml:"services"`
    FaultInjection FaultConfig        `yaml:"fault_injection"`
    Telemetry TelemetryConfig         `yaml:"telemetry"`
}

type ServiceConfig struct {
    Port int `yaml:"port"`
}

type FaultConfig struct {
    PaymentFailureRate  float64 `yaml:"payment_failure_rate"`
    ShippingFailureRate float64 `yaml:"shipping_failure_rate"`
}

type TelemetryConfig struct {
    Enabled     bool   `yaml:"enabled"`
    ServiceName string `yaml:"service_name"`
}
```

**Functions Required**:
- `LoadConfig(filepath string) (*Config, error)` - Load YAML file
- `GetServicePort(serviceName string) int` - Get port for service

**Testing**:
- Unit test with sample YAML file
- Test error handling for missing files
- Test malformed YAML handling

### 2. Database Component (`database.go`)

**Purpose**: Simple in-memory storage with thread safety

**Implementation Requirements**:
```go
type MockDB struct {
    mu   sync.RWMutex
    data map[string]interface{}
}
```

**Functions Required**:
- `NewMockDB() *MockDB` - Constructor
- `Set(key string, value interface{}) error` - Store data
- `Get(key string) (interface{}, error)` - Retrieve data
- `Delete(key string) error` - Remove data
- `Exists(key string) bool` - Check existence

**Error Handling**:
- Return `ErrNotFound` for missing keys
- Return `ErrInvalidKey` for empty keys

**Testing**:
- Concurrent read/write tests
- Basic CRUD operations
- Error condition tests

### 3. Middleware Component (`middleware.go`)

**Purpose**: HTTP middleware for logging, tracing, and correlation IDs

**Implementation Requirements**:
```go
// Middleware functions that return http.Handler
func LoggingMiddleware(next http.Handler) http.Handler
func TracingMiddleware(next http.Handler) http.Handler
func CorrelationIDMiddleware(next http.Handler) http.Handler
```

**Logging Middleware**:
- Log method, path, status code, duration
- Use standard Go log package
- Format: `[timestamp] method=GET path=/api/products status=200 duration=15ms`

**Tracing Middleware**:
- Create OpenTelemetry span for each request
- Set span attributes: `http.method`, `http.url`, `http.status_code`
- Handle span lifecycle properly

**Correlation ID Middleware**:
- Extract from `X-Correlation-ID` header or generate UUID
- Add to request context
- Add to response header
- Pass to OpenTelemetry span

**Testing**:
- Test middleware chain execution order
- Verify headers are set correctly
- Test span creation and attributes

### 4. Error Types Component (`errors.go`)

**Purpose**: Standardized error types across all services

**Implementation Requirements**:
```go
type AppError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e AppError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
```

**Predefined Errors**:
```go
var (
    ErrNotFound     = AppError{Code: "NOT_FOUND", Message: "Resource not found"}
    ErrBadRequest   = AppError{Code: "BAD_REQUEST", Message: "Invalid request"}
    ErrInternal     = AppError{Code: "INTERNAL_ERROR", Message: "Internal server error"}
    ErrInvalidKey   = AppError{Code: "INVALID_KEY", Message: "Key cannot be empty"}
)
```

**Functions Required**:
- `NewAppError(code, message string) AppError` - Create custom error
- `WriteErrorResponse(w http.ResponseWriter, err AppError, statusCode int)` - HTTP error response

**Testing**:
- Test error serialization to JSON
- Test HTTP error response formatting
- Test error equality comparisons

### 5. Telemetry Component (`telemetry.go`)

**Purpose**: Basic OpenTelemetry setup for HTTP tracing only

**Implementation Requirements**:
```go
func InitTelemetry(serviceName string) (func(), error) {
    // Returns cleanup function and error
}

func GetTracer() trace.Tracer {
    // Return global tracer instance
}
```

**Setup Requirements**:
- Console span exporter (not OTLP)
- HTTP auto-instrumentation
- Simple span processor (not batch)
- Resource with service name

**Context Utilities**:
```go
func GetCorrelationID(ctx context.Context) string
func SetCorrelationID(ctx context.Context, id string) context.Context
```

**Testing**:
- Test telemetry initialization
- Test span creation and export
- Test context utilities

### 6. Models Component (`models.go`)

**Purpose**: Shared data structures used across services

**Implementation Requirements**:
```go
type Product struct {
    ID       string  `json:"id" yaml:"id"`
    Name     string  `json:"name" yaml:"name"`
    Price    float64 `json:"price" yaml:"price"`
    Stock    int     `json:"stock" yaml:"stock"`
    Category string  `json:"category" yaml:"category"`
    ImageURL string  `json:"image_url" yaml:"image_url"`
}

type CartItem struct {
    ProductID string  `json:"product_id"`
    Quantity  int     `json:"quantity"`
    Price     float64 `json:"price"`
}

type Cart struct {
    ID         string     `json:"id"`
    CustomerID string     `json:"customer_id"`
    Items      []CartItem `json:"items"`
    Total      float64    `json:"total"`
}

type Order struct {
    ID         string     `json:"id"`
    CustomerID string     `json:"customer_id"`
    Items      []CartItem `json:"items"`
    Total      float64    `json:"total"`
    Status     string     `json:"status"` // "pending", "paid", "shipped", "failed"
}

type Transaction struct {
    ID        string  `json:"id"`
    OrderID   string  `json:"order_id"`
    Amount    float64 `json:"amount"`
    Status    string  `json:"status"` // "success", "failed"
    Processor string  `json:"processor"`
}

type Shipment struct {
    ID      string  `json:"id"`
    OrderID string  `json:"order_id"`
    Carrier string  `json:"carrier"`
    Status  string  `json:"status"` // "submitted", "failed"
    Cost    float64 `json:"cost"`
}
```

**Validation Functions**:
```go
func (p Product) Validate() error
func (c Cart) CalculateTotal() float64
```

**Testing**:
- Test JSON serialization/deserialization
- Test validation functions
- Test total calculation

## Service Integration

### How Other Services Use Common

**Import Pattern**:
```go
import "github.com/cardinalhq/griffin-commerce-demo/common"
```

**Configuration Usage**:
```go
config, err := common.LoadConfig("config.yaml")
if err != nil {
    log.Fatal(err)
}
port := config.GetServicePort("catalog")
```

**Database Usage**:
```go
db := common.NewMockDB()
err := db.Set("product:123", product)
data, err := db.Get("product:123")
```

**Middleware Usage**:
```go
r := mux.NewRouter()
r.Use(common.LoggingMiddleware)
r.Use(common.TracingMiddleware)
r.Use(common.CorrelationIDMiddleware)
```

**Error Handling**:
```go
if err != nil {
    common.WriteErrorResponse(w, common.ErrNotFound, http.StatusNotFound)
    return
}
```

**Telemetry Usage**:
```go
cleanup, err := common.InitTelemetry("catalog-service")
defer cleanup()

tracer := common.GetTracer()
ctx, span := tracer.Start(ctx, "get-product")
defer span.End()
```

## File Structure

```
common/
├── config.go      (~100 lines)
├── database.go    (~80 lines)
├── middleware.go  (~120 lines)
├── errors.go      (~60 lines)
├── telemetry.go   (~80 lines)
└── models.go      (~150 lines)
```

**Total**: ~590 lines across 6 files

## Dependencies

### Go Standard Library
- `encoding/json`
- `fmt`
- `log`
- `net/http`
- `sync`
- `context`
- `gopkg.in/yaml.v2`

### External Dependencies
- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/trace`
- `go.opentelemetry.io/otel/sdk/trace`
- `go.opentelemetry.io/otel/exporters/stdout/stdouttrace`
- `github.com/google/uuid` (for correlation IDs)

## Testing Strategy

### Unit Tests Required

**config_test.go**:
- Test YAML loading with valid config
- Test error handling for missing files
- Test malformed YAML handling

**database_test.go**:
- Test concurrent access safety
- Test CRUD operations
- Test error conditions

**middleware_test.go**:
- Test middleware chain execution
- Test correlation ID generation/extraction
- Test logging output format

**errors_test.go**:
- Test error JSON serialization
- Test HTTP error response formatting

**telemetry_test.go**:
- Test initialization and cleanup
- Test span creation

**models_test.go**:
- Test JSON marshaling/unmarshaling
- Test validation functions
- Test cart total calculation

### Test Coverage Target
- Minimum 80% code coverage
- All error paths must be tested
- All public functions must have tests

## Implementation Order

1. **Phase 1**: Core Infrastructure
   - `errors.go` - Foundation error types
   - `config.go` - Configuration loading
   - `models.go` - Data structures

2. **Phase 2**: Storage and Middleware
   - `database.go` - In-memory storage
   - `middleware.go` - HTTP middleware

3. **Phase 3**: Observability
   - `telemetry.go` - OpenTelemetry setup

4. **Phase 4**: Testing
   - Complete unit test suite
   - Integration testing with sample service

## Success Criteria

The Common Service is complete when:
1. All 6 components are implemented and tested
2. Other services can import and use all utilities
3. HTTP middleware works correctly with tracing
4. Configuration loads from YAML successfully
5. In-memory database handles concurrent access safely
6. Error handling is consistent across all components
7. OpenTelemetry tracing captures HTTP requests properly

## Constraints and Limitations

### What This Design Does NOT Include
- Caching layers of any kind
- Circuit breakers or advanced fault tolerance
- Hot configuration reload
- Complex retry logic
- Database transactions or persistence
- Query capabilities beyond basic CRUD
- Custom metrics or advanced telemetry
- Authentication or authorization
- Rate limiting
- Request validation beyond basic error checking

### Intentional Simplifications
- No interfaces - direct struct usage
- No generics - simple types only
- No advanced concurrency patterns
- No optimization for performance
- Minimal error handling
- Basic logging without structured formats
- Console exporter only for traces

This design prioritizes simplicity and speed of implementation over production-readiness, perfectly suited for a 2-week POC demonstration.