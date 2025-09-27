# Common Service System Design

## Overview

The Common Service provides shared utilities, middleware, and infrastructure components for the Griffin Commerce POC e-commerce platform. It serves as the foundation layer that all other services depend on, providing consistent patterns for configuration, observability, data access, and error handling.

## Architecture Principles

- **POC-First**: All components are mock/in-memory implementations with no external dependencies
- **Observable**: Full OpenTelemetry instrumentation with structured logging
- **Testable**: Comprehensive testing utilities and mock generators
- **Configurable**: YAML-based configuration with hot-reload capabilities
- **Fault-Tolerant**: Built-in fault injection framework for resilience testing

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Common Service                          │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────┤
│ Config      │ Mock DB     │ Mock Cache  │ Telemetry   │ HTTP    │
│ Management  │ Layer       │ Layer       │ & Logging   │ Stack   │
├─────────────┼─────────────┼─────────────┼─────────────┼─────────┤
│ Fault       │ Security    │ Models      │ Testing     │ Health  │
│ Injection   │ Utilities   │ & Errors    │ Utilities   │ Monitor │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────┘
```

## File Structure

```
/common
├── config/                     # Configuration management
│   ├── config.go              # Core configuration types
│   ├── loader.go              # YAML configuration loader
│   ├── validator.go           # Configuration validation
│   └── watcher.go             # Hot-reload configuration watcher
├── mockdb/                     # Mock database layer
│   ├── interface.go           # Database interface definition
│   ├── memory.go              # In-memory implementation
│   ├── persistence.go         # JSON persistence layer
│   ├── transaction.go         # Transaction simulation
│   └── query.go               # Query filtering and sorting
├── mockcache/                  # Mock cache layer
│   ├── interface.go           # Cache interface definition
│   ├── memory.go              # In-memory LRU cache
│   ├── ttl.go                 # TTL expiration manager
│   ├── stats.go               # Cache statistics
│   └── warming.go             # Cache warming from YAML
├── fault/                      # Fault injection framework
│   ├── injector.go            # Core fault injection logic
│   ├── config.go              # Fault configuration types
│   ├── rules.go               # Pattern-based failure rules
│   └── middleware.go          # HTTP middleware integration
├── telemetry/                  # OpenTelemetry setup
│   ├── tracer.go              # Tracer configuration
│   ├── metrics.go             # Metrics collection
│   ├── propagation.go         # Context propagation
│   └── exporters.go           # OTLP exporters
├── logging/                    # Structured logging
│   ├── logger.go              # Core logger with OTEL integration
│   ├── correlation.go         # Correlation ID management
│   ├── middleware.go          # HTTP logging middleware
│   └── formatter.go           # JSON log formatting
├── http/                       # HTTP middleware stack
│   ├── middleware.go          # Middleware chain builder
│   ├── auth.go                # Authentication middleware
│   ├── cors.go                # CORS configuration
│   ├── compression.go         # Request/response compression
│   ├── ratelimit.go           # Rate limiting
│   ├── validation.go          # Request validation
│   ├── timeout.go             # Timeout handling
│   └── recovery.go            # Panic recovery
├── errors/                     # Error handling framework
│   ├── types.go               # Custom error types with codes
│   ├── wrapper.go             # Error wrapping with context
│   ├── http.go                # HTTP error response formatting
│   └── recovery.go            # Error recovery strategies
├── security/                   # Security utilities
│   ├── jwt.go                 # JWT token management
│   ├── hash.go                # Password hashing (bcrypt)
│   ├── apikey.go              # API key management
│   ├── hmac.go                # HMAC signature verification
│   └── sanitize.go            # Input sanitization
├── models/                     # Shared data models
│   ├── customer.go            # Customer entity
│   ├── product.go             # Product entity
│   ├── order.go               # Order entity
│   ├── address.go             # Address entity
│   ├── money.go               # Money type with currency
│   └── timestamp.go           # Timestamp utilities
├── testing/                    # Testing utilities
│   ├── setup.go               # Test environment setup
│   ├── generators.go          # Mock data generators
│   ├── client.go              # HTTP test client
│   ├── assertions.go          # Custom test assertions
│   └── helpers.go             # Integration test helpers
├── monitoring/                 # Health and monitoring
│   ├── health.go              # Health check endpoints
│   ├── readiness.go           # Readiness probes
│   ├── metrics.go             # Service metrics
│   └── debug.go               # Debug endpoints
└── validation/                 # Validation utilities
    ├── validator.go           # Input validation framework
    ├── rules.go               # Validation rules
    ├── sanitize.go            # Data sanitization
    └── schema.go              # JSON schema validation
```

## Component Specifications

### 1. Configuration Management (`/config`)

**Purpose**: Centralized configuration loading, validation, and hot-reload capability.

**Core Types**:
```go
type Config struct {
    App         AppConfig         `yaml:"app"`
    Telemetry   TelemetryConfig   `yaml:"telemetry"`
    Services    ServicesConfig    `yaml:"services"`
    FaultInjection FaultConfig    `yaml:"fault_injection"`
    Database    DatabaseConfig    `yaml:"database"`
    Cache       CacheConfig       `yaml:"cache"`
}

type AppConfig struct {
    Name        string `yaml:"name" validate:"required"`
    Environment string `yaml:"environment" validate:"required,oneof=poc dev staging prod"`
    Port        int    `yaml:"port" validate:"required,min=1024,max=65535"`
    LogLevel    string `yaml:"log_level" validate:"required,oneof=debug info warn error fatal"`
}
```

**Key Features**:
- YAML configuration loading with environment variable substitution
- Configuration validation using struct tags
- Hot-reload capability with file system watching
- Service-specific configuration namespaces
- Feature flag and toggle support

**Test Strategy**:
- Unit tests for configuration validation
- Integration tests for hot-reload functionality
- Test fixtures for different configuration scenarios

### 2. Mock Database Layer (`/mockdb`)

**Purpose**: In-memory database simulation with thread-safe operations and optional persistence.

**Core Interface**:
```go
type MockDB interface {
    Create(ctx context.Context, entity interface{}) error
    Read(ctx context.Context, id string, entity interface{}) error
    Update(ctx context.Context, id string, entity interface{}) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter FilterOptions) ([]interface{}, error)
    Query(ctx context.Context, query QueryOptions) ([]interface{}, error)
    BeginTransaction(ctx context.Context) (Transaction, error)
    Close() error
}

type Transaction interface {
    Create(ctx context.Context, entity interface{}) error
    Update(ctx context.Context, id string, entity interface{}) error
    Delete(ctx context.Context, id string) error
    Commit() error
    Rollback() error
}

type FilterOptions struct {
    Fields    map[string]interface{}
    SortBy    string
    SortOrder SortOrder
    Limit     int
    Offset    int
}
```

**Implementation Details**:
- Thread-safe operations using `sync.RWMutex`
- Generic CRUD operations with reflection-based entity handling
- JSON persistence layer for data recovery across restarts
- Transaction simulation with rollback capability
- Configurable latency injection for realistic testing
- Query filtering and sorting support
- Bulk operation support for performance testing

**Test Strategy**:
- Concurrent access tests to verify thread safety
- Transaction rollback scenario tests
- Performance tests with configurable latency injection
- Data persistence and recovery tests

### 3. Mock Cache Layer (`/mockcache`)

**Purpose**: In-memory cache with TTL support, LRU eviction, and statistics tracking.

**Core Interface**:
```go
type MockCache interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Keys(ctx context.Context, pattern string) ([]string, error)
    Stats() CacheStats
    WarmFromYAML(filename string) error
}

type CacheStats struct {
    Hits        int64
    Misses      int64
    Evictions   int64
    KeyCount    int64
    MemoryBytes int64
}
```

**Implementation Details**:
- LRU eviction policy when memory limits are reached
- TTL support with automatic background expiration
- Thread-safe concurrent access using `sync.RWMutex`
- Pattern matching for bulk operations
- Cache warming from YAML configuration files
- Memory usage tracking and limits
- Detailed statistics for monitoring and debugging

**Test Strategy**:
- TTL expiration tests with time mocking
- LRU eviction behavior verification
- Concurrent access stress tests
- Memory limit enforcement tests
- Cache warming functionality tests

### 4. Fault Injection Framework (`/fault`)

**Purpose**: Configurable fault injection for resilience testing across all services.

**Core Interface**:
```go
type FaultInjector interface {
    ShouldFail(ctx context.Context, operation string, request interface{}) (bool, FaultType, error)
    InjectLatency(ctx context.Context, operation string) time.Duration
    UpdateConfig(config FaultConfig) error
    GetStats() FaultStats
}

type FaultConfig struct {
    Services map[string]ServiceFaultConfig `yaml:"services"`
}

type ServiceFaultConfig struct {
    FailureRate    float64           `yaml:"failure_rate"`
    FailureType    string            `yaml:"failure_type"`
    LatencyMS      int               `yaml:"latency_ms"`
    SpecificRules  []FaultRule       `yaml:"specific_rules"`
}

type FaultRule struct {
    Match        map[string]interface{} `yaml:"match"`
    FailureRate  float64               `yaml:"failure_rate"`
    FailureType  string                `yaml:"failure_type"`
    Error        string                `yaml:"error"`
}
```

**Implementation Details**:
- Service-level and operation-level fault configuration
- Pattern-based failure rules for specific scenarios
- Runtime configuration updates without service restart
- HTTP middleware integration for automatic fault injection
- Comprehensive fault statistics and reporting
- Multiple fault types: timeouts, errors, partial failures, latency injection

**Test Strategy**:
- Fault injection accuracy tests (verify configured failure rates)
- Runtime configuration update tests
- Pattern matching rule verification
- Integration tests with HTTP middleware

### 5. OpenTelemetry Setup (`/telemetry`)

**Purpose**: Complete observability setup with tracing, metrics, and logging integration.

**Core Interface**:
```go
type TelemetryProvider interface {
    GetTracer(name string) trace.Tracer
    GetMeter(name string) metric.Meter
    RecordMetric(ctx context.Context, name string, value float64, labels map[string]string)
    CreateSpan(ctx context.Context, name string) (context.Context, trace.Span)
    Shutdown(ctx context.Context) error
}

type TelemetryConfig struct {
    Enabled       bool    `yaml:"enabled"`
    ServiceName   string  `yaml:"service_name"`
    OTLPEndpoint  string  `yaml:"otlp_endpoint"`
    SamplingRate  float64 `yaml:"sampling_rate"`
}
```

**Implementation Details**:
- OTLP exporter configuration for traces and metrics
- Automatic HTTP middleware instrumentation
- Context propagation across service boundaries
- Custom business metrics collection
- Correlation ID integration with logging
- Resource detection and service identification
- Configurable sampling rates

**Automatic Metrics**:
- `http_request_duration_seconds`
- `http_request_size_bytes`
- `http_response_size_bytes`
- `http_requests_total`
- `database_operation_duration_seconds`
- `cache_operation_duration_seconds`
- `fault_injection_triggered_total`

**Test Strategy**:
- Trace creation and propagation tests
- Metric recording accuracy tests
- OTLP exporter integration tests
- Context propagation verification

### 6. HTTP Middleware Stack (`/http`)

**Purpose**: Comprehensive HTTP middleware for authentication, logging, rate limiting, and error handling.

**Middleware Chain**:
1. **Recovery Middleware**: Panic recovery with error logging
2. **Correlation ID**: Request ID generation and propagation
3. **Logging Middleware**: Request/response logging with telemetry
4. **Authentication**: JWT validation and user context
5. **Authorization**: Role-based access control
6. **CORS**: Cross-origin request handling
7. **Rate Limiting**: Per-endpoint rate limiting
8. **Compression**: Request/response compression (gzip)
9. **Validation**: Request payload validation
10. **Timeout**: Request timeout handling
11. **Fault Injection**: Automatic fault injection integration

**Core Interface**:
```go
type MiddlewareChain interface {
    Use(middleware Middleware) MiddlewareChain
    Build() http.Handler
}

type Middleware func(http.Handler) http.Handler

type AuthMiddleware interface {
    ValidateJWT(next http.Handler) http.Handler
    RequireRole(role string) Middleware
    OptionalAuth(next http.Handler) http.Handler
}
```

**Test Strategy**:
- Individual middleware unit tests
- Middleware chain integration tests
- Authentication and authorization flow tests
- Rate limiting behavior verification
- Error handling and recovery tests

### 7. Logging Framework (`/logging`)

**Purpose**: Structured logging with OpenTelemetry integration and correlation ID propagation.

**Core Interface**:
```go
type Logger interface {
    Debug(ctx context.Context, msg string, fields ...Field)
    Info(ctx context.Context, msg string, fields ...Field)
    Warn(ctx context.Context, msg string, fields ...Field)
    Error(ctx context.Context, msg string, fields ...Field)
    Fatal(ctx context.Context, msg string, fields ...Field)
    With(fields ...Field) Logger
}

type Field struct {
    Key   string
    Value interface{}
}
```

**Features**:
- Structured JSON logging with consistent format
- Automatic trace ID and span ID injection
- Correlation ID generation and propagation
- Context-aware logging with request metadata
- Log level configuration and filtering
- Performance-optimized to not block request processing

**Log Format**:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "info",
  "service": "cart-service",
  "trace_id": "abc123...",
  "span_id": "def456...",
  "correlation_id": "req-789...",
  "message": "Processing cart update",
  "fields": {
    "user_id": "user123",
    "cart_id": "cart456",
    "operation": "add_item"
  }
}
```

**Test Strategy**:
- Log format validation tests
- Correlation ID propagation tests
- Performance benchmarks for logging overhead
- Context integration tests

### 8. Error Handling Framework (`/errors`)

**Purpose**: Standardized error handling with custom error types and HTTP response formatting.

**Core Types**:
```go
type ErrorCode string

const (
    ErrorCodeValidation    ErrorCode = "VALIDATION_ERROR"
    ErrorCodeNotFound      ErrorCode = "NOT_FOUND"
    ErrorCodeUnauthorized  ErrorCode = "UNAUTHORIZED"
    ErrorCodeInternal      ErrorCode = "INTERNAL_ERROR"
    ErrorCodeRateLimit     ErrorCode = "RATE_LIMIT_EXCEEDED"
    ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

type AppError struct {
    Code       ErrorCode              `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    Internal   error                  `json:"-"`
    StackTrace string                 `json:"-"`
}

type HTTPErrorResponse struct {
    Error     AppError `json:"error"`
    RequestID string   `json:"request_id"`
    Timestamp string   `json:"timestamp"`
}
```

**Features**:
- Custom error types with error codes
- Error wrapping with context preservation
- HTTP error response standardization
- Stack trace capture for debugging
- User-friendly error message formatting
- Error recovery strategies and circuit breakers

**Test Strategy**:
- Error type creation and wrapping tests
- HTTP error response format validation
- Error recovery mechanism tests
- Stack trace capture verification

### 9. Shared Data Models (`/models`)

**Purpose**: Common data structures used across all services.

**Core Models**:
```go
type Customer struct {
    ID        string    `json:"id" db:"id"`
    Email     string    `json:"email" db:"email" validate:"required,email"`
    Name      string    `json:"name" db:"name" validate:"required"`
    Addresses []Address `json:"addresses" db:"addresses"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Product struct {
    ID          string            `json:"id" db:"id"`
    SKU         string            `json:"sku" db:"sku" validate:"required"`
    Name        string            `json:"name" db:"name" validate:"required"`
    Description string            `json:"description" db:"description"`
    Category    string            `json:"category" db:"category" validate:"required"`
    Price       Money             `json:"price" db:"price"`
    Images      []ProductImage    `json:"images" db:"images"`
    Inventory   int               `json:"inventory" db:"inventory"`
    Tags        []string          `json:"tags" db:"tags"`
    Metadata    map[string]string `json:"metadata" db:"metadata"`
}

type Money struct {
    Amount   decimal.Decimal `json:"amount" db:"amount"`
    Currency string          `json:"currency" db:"currency" validate:"required,len=3"`
}
```

**Features**:
- Validation tags for input validation
- JSON and database serialization tags
- Consistent ID generation patterns
- Timezone-aware timestamp handling
- Money type with currency support and decimal precision

**Test Strategy**:
- Model validation tests
- JSON serialization/deserialization tests
- Database mapping tests
- Money type arithmetic tests

### 10. Testing Utilities (`/testing`)

**Purpose**: Comprehensive testing helpers and mock data generators.

**Core Interface**:
```go
type TestEnvironment interface {
    SetupDatabase() MockDB
    SetupCache() MockCache
    SetupHTTPClient() *http.Client
    SetupTelemetry() TelemetryProvider
    Cleanup()
}

type DataGenerator interface {
    GenerateCustomer() *Customer
    GenerateProduct() *Product
    GenerateOrder() *Order
    GenerateCustomers(count int) []*Customer
    GenerateProducts(count int) []*Product
}
```

**Features**:
- Test database setup and teardown
- Realistic mock data generation using faker library
- HTTP test client with authentication
- Integration test helpers for service communication
- Telemetry assertion helpers
- Benchmark utilities for performance testing
- Test scenario loading from YAML files

**Test Strategy**:
- Mock data generation quality tests
- Test environment setup/teardown verification
- HTTP client functionality tests
- Integration test helper validation

## Data Flow Architecture

### Request Processing Flow
```
1. HTTP Request → Recovery Middleware
2. Recovery → Correlation ID Middleware
3. Correlation ID → Logging Middleware (start)
4. Logging → Authentication Middleware
5. Authentication → CORS Middleware
6. CORS → Rate Limiting Middleware
7. Rate Limiting → Validation Middleware
8. Validation → Fault Injection Middleware
9. Fault Injection → Business Logic Handler
10. Business Logic → Telemetry Recording
11. Response → Logging Middleware (end)
```

### Configuration Loading Flow
```
1. Application Start → Config Loader
2. Config Loader → YAML File Parser
3. YAML Parser → Environment Variable Substitution
4. Environment Variables → Configuration Validation
5. Validation → Configuration Object Creation
6. Configuration → Service Initialization
7. File System Watcher → Hot Reload Detection
8. Hot Reload → Configuration Re-validation
9. Re-validation → Service Configuration Update
```

### Observability Data Flow
```
1. HTTP Request → Span Creation
2. Span Creation → Correlation ID Injection
3. Database Operation → Span Recording
4. Cache Operation → Metric Recording
5. Error Occurrence → Error Span Recording
6. Request Completion → Span Finalization
7. Span Data → OTLP Exporter
8. OTLP Exporter → Telemetry Backend
```

## Testing Strategy

### Unit Testing Requirements
- **Configuration**: Validation, loading, hot-reload
- **Mock Database**: CRUD operations, transactions, concurrency
- **Mock Cache**: TTL expiration, LRU eviction, statistics
- **Fault Injection**: Rule matching, failure rate accuracy
- **Telemetry**: Metric recording, span creation, context propagation
- **HTTP Middleware**: Individual middleware behavior, chain composition
- **Error Handling**: Error type creation, HTTP response formatting
- **Models**: Validation, serialization, business logic

### Integration Testing Requirements
- **End-to-End Request Flow**: Full middleware chain processing
- **Service Communication**: Inter-service calls with telemetry
- **Configuration Hot-Reload**: Live configuration updates
- **Fault Injection**: Runtime fault injection activation
- **Database Persistence**: JSON persistence and recovery
- **Cache Warming**: YAML-based cache initialization

### Performance Testing Requirements
- **Middleware Overhead**: < 1ms per request target
- **Cache Operations**: < 10ms target for all operations
- **Database Operations**: Configurable latency simulation
- **Memory Usage**: Cache memory limits and tracking
- **Concurrent Access**: Thread safety under load

## Implementation Order

### Phase 1: Foundation (Week 1)
1. Configuration management system
2. Logging framework with correlation IDs
3. Basic error handling framework
4. Shared data models

### Phase 2: Data Layer (Week 2)
1. Mock database interface and implementation
2. Mock cache interface and implementation
3. Data persistence layer
4. Query and filtering capabilities

### Phase 3: Observability (Week 3)
1. OpenTelemetry setup and configuration
2. Telemetry middleware integration
3. Metrics collection framework
4. Distributed tracing implementation

### Phase 4: HTTP Stack (Week 4)
1. Core middleware implementations
2. Authentication and authorization
3. Rate limiting and validation
4. Middleware chain composition

### Phase 5: Advanced Features (Week 5)
1. Fault injection framework
2. Security utilities
3. Testing utilities and helpers
4. Health and monitoring endpoints

## Dependencies

### External Libraries
- `go.opentelemetry.io/otel` - OpenTelemetry instrumentation
- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/golang-jwt/jwt/v5` - JWT token handling
- `golang.org/x/crypto/bcrypt` - Password hashing
- `github.com/shopspring/decimal` - Decimal arithmetic for money
- `github.com/go-playground/validator/v10` - Struct validation
- `github.com/fsnotify/fsnotify` - File system watching
- `github.com/brianvoe/gofakeit/v6` - Mock data generation

### Internal Dependencies
- No dependencies on other services (foundation layer)
- All services depend on common package
- Circular dependency prevention through interfaces

## Performance Requirements

### Response Time Targets
- Configuration loading: < 100ms on startup
- Middleware processing: < 1ms per request
- Cache operations: < 10ms
- Database operations: Configurable (default: 5ms)
- Fault injection decision: < 1ms

### Throughput Targets
- HTTP middleware: Handle 10,000+ requests/second
- Cache operations: 100,000+ operations/second
- Database operations: 50,000+ operations/second
- Telemetry recording: No blocking of request processing

### Resource Limits
- Memory usage: Cache-configurable limits (default: 100MB)
- CPU overhead: < 5% for common operations
- Goroutine usage: Bounded and monitored
- File descriptors: Minimal usage, proper cleanup

## Security Considerations

### Authentication & Authorization
- JWT token validation with configurable signing keys
- Role-based access control middleware
- API key management for service-to-service communication
- Token expiration and refresh handling

### Input Validation & Sanitization
- Comprehensive input validation using struct tags
- XSS prevention helpers
- SQL injection prevention (even for mock database)
- Request size limits and timeout enforcement

### Security Headers
- CORS configuration with proper origin validation
- Security headers middleware (HSTS, CSP, etc.)
- Request rate limiting per client/endpoint
- HMAC signature verification for webhook endpoints

## Monitoring & Alerting

### Health Endpoints
- `/health` - Basic service health check
- `/ready` - Readiness probe for Kubernetes
- `/debug/config` - Current configuration (sanitized)
- `/debug/mock-db` - Mock database inspection
- `/metrics` - Prometheus metrics endpoint

### Key Metrics to Monitor
- Request latency percentiles (p50, p95, p99)
- Error rates by endpoint and error type
- Cache hit rates and eviction frequency
- Database operation latencies
- Fault injection trigger rates
- Memory usage and garbage collection metrics

### Alerting Thresholds
- Error rate > 5% for any endpoint
- Request latency p95 > 500ms
- Cache hit rate < 80%
- Memory usage > 80% of limit
- Any FATAL log entries

This comprehensive design provides a solid foundation for the Griffin Commerce POC, emphasizing observability, testability, and fault tolerance while maintaining simplicity and avoiding external dependencies.