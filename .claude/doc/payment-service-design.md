# Payment Service System Design

## Overview

The Payment Service is a core component of the Griffin Commerce demo e-commerce platform, responsible for processing payments, managing transactions, and handling refunds. This service supports multiple payment processors and includes comprehensive fault injection capabilities for testing resilience.

## Architecture Overview

### High-Level Components

```text
┌─────────────────────────────────────────────────────────────┐
│                    Payment Service                          │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │
│  │   HTTP API  │  │  Webhook     │  │  Admin/Debug    │    │
│  │   Handler   │  │  Handler     │  │  Endpoints      │    │
│  └─────────────┘  └──────────────┘  └─────────────────┘    │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │
│  │ Transaction │  │   Payment    │  │  Fraud/Rate     │    │
│  │   Manager   │  │   Processor  │  │   Limiter       │    │
│  └─────────────┘  └──────────────┘  └─────────────────┘    │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │
│  │   Mock      │  │    Fault     │  │  OpenTelemetry  │    │
│  │ Processors  │  │  Injection   │  │ Instrumentation │    │
│  └─────────────┘  └──────────────┘  └─────────────────┘    │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐    │
│  │   Mock DB   │  │  Token Store │  │   Config        │    │
│  │  (Memory)   │  │  (Memory)    │  │   Manager       │    │
│  └─────────────┘  └──────────────┘  └─────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```text
payment/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point and server setup
├── internal/
│   ├── api/
│   │   ├── handlers.go               # HTTP request handlers
│   │   ├── middleware.go             # Authentication, logging, rate limiting
│   │   ├── routes.go                 # Route definitions
│   │   └── responses.go              # Response models and helpers
│   ├── domain/
│   │   ├── transaction.go            # Transaction entity and business logic
│   │   ├── payment_method.go         # Payment method entity
│   │   ├── refund.go                 # Refund entity
│   │   └── errors.go                 # Domain-specific errors
│   ├── services/
│   │   ├── transaction_service.go    # Transaction business logic
│   │   ├── payment_service.go        # Payment processing orchestration
│   │   ├── refund_service.go         # Refund processing logic
│   │   └── token_service.go          # Payment method tokenization
│   ├── processors/
│   │   ├── interface.go              # Payment processor interface
│   │   ├── factory.go                # Processor factory
│   │   ├── puppypay.go               # PuppyPay mock processor
│   │   ├── kittycard.go              # KittyCard mock processor
│   │   ├── doggiecoin.go             # DoggieCoin mock processor
│   │   └── pawpal.go                 # PawPal mock processor
│   ├── repository/
│   │   ├── interface.go              # Repository interfaces
│   │   ├── transaction_repo.go       # Transaction repository
│   │   ├── payment_method_repo.go    # Payment method repository
│   │   └── mockdb/
│   │       ├── memory_db.go          # In-memory database implementation
│   │       └── json_persistence.go  # Optional JSON file persistence
│   ├── fault/
│   │   ├── injector.go               # Fault injection engine
│   │   ├── config.go                 # Fault configuration
│   │   └── patterns.go               # Specific failure patterns
│   ├── telemetry/
│   │   ├── instrumentation.go        # OpenTelemetry setup
│   │   ├── metrics.go                # Custom metrics
│   │   └── traces.go                 # Tracing helpers
│   └── config/
│       ├── config.go                 # Configuration structures
│       └── validation.go             # Configuration validation
├── pkg/
│   └── client/
│       ├── client.go                 # HTTP client for payment service
│       └── models.go                 # Request/response models
├── configs/
│   ├── config.yaml                   # Main configuration
│   ├── fault_injection.yaml          # Fault injection rules
│   └── test_scenarios.yaml           # Test data scenarios
└── test/
    ├── integration/
    │   ├── payment_flow_test.go      # End-to-end payment tests
    │   ├── fault_injection_test.go   # Fault injection tests
    │   └── webhook_test.go           # Webhook handling tests
    └── unit/
        ├── handlers_test.go          # API handler tests
        ├── services_test.go          # Service layer tests
        ├── processors_test.go        # Processor tests
        └── repository_test.go        # Repository tests
```

## Data Models

### Core Entities

#### Transaction

```go
type Transaction struct {
    ID              string                 `json:"id" db:"id"`
    OrderID         string                 `json:"order_id" db:"order_id"`
    CustomerID      string                 `json:"customer_id" db:"customer_id"`
    Amount          decimal.Decimal        `json:"amount" db:"amount"`
    Currency        string                 `json:"currency" db:"currency"`
    Status          TransactionStatus      `json:"status" db:"status"`
    PaymentMethod   PaymentMethodType      `json:"payment_method" db:"payment_method"`
    ProcessorName   string                 `json:"processor_name" db:"processor_name"`
    ProcessorTxnID  string                 `json:"processor_txn_id" db:"processor_txn_id"`
    IdempotencyKey  string                 `json:"idempotency_key" db:"idempotency_key"`
    AuthCode        string                 `json:"auth_code,omitempty" db:"auth_code"`
    Metadata        map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
    CreatedAt       time.Time              `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at" db:"updated_at"`
    AuthorizedAt    *time.Time             `json:"authorized_at,omitempty" db:"authorized_at"`
    CapturedAt      *time.Time             `json:"captured_at,omitempty" db:"captured_at"`
    FailedAt        *time.Time             `json:"failed_at,omitempty" db:"failed_at"`
    FailureReason   string                 `json:"failure_reason,omitempty" db:"failure_reason"`
}

type TransactionStatus string

const (
    StatusPending    TransactionStatus = "pending"
    StatusAuthorized TransactionStatus = "authorized"
    StatusCaptured   TransactionStatus = "captured"
    StatusRefunded   TransactionStatus = "refunded"
    StatusFailed     TransactionStatus = "failed"
    StatusCanceled   TransactionStatus = "canceled"
)
```

#### PaymentMethod

```go
type PaymentMethod struct {
    ID         string            `json:"id" db:"id"`
    CustomerID string            `json:"customer_id" db:"customer_id"`
    Token      string            `json:"token" db:"token"`
    Type       PaymentMethodType `json:"type" db:"type"`
    Last4      string            `json:"last4" db:"last4"`
    ExpiryDate string            `json:"expiry_date,omitempty" db:"expiry_date"`
    Brand      string            `json:"brand,omitempty" db:"brand"`
    IsDefault  bool              `json:"is_default" db:"is_default"`
    CreatedAt  time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time         `json:"updated_at" db:"updated_at"`
}

type PaymentMethodType string

const (
    TypeCreditCard   PaymentMethodType = "credit_card"
    TypeDebitCard    PaymentMethodType = "debit_card"
    TypeDigitalWallet PaymentMethodType = "digital_wallet"
    TypeCrypto       PaymentMethodType = "crypto"
)
```

#### RefundRecord
```go
type RefundRecord struct {
    ID            string          `json:"id" db:"id"`
    TransactionID string          `json:"transaction_id" db:"transaction_id"`
    Amount        decimal.Decimal `json:"amount" db:"amount"`
    Currency      string          `json:"currency" db:"currency"`
    Reason        string          `json:"reason" db:"reason"`
    Status        RefundStatus    `json:"status" db:"status"`
    ProcessorRefundID string      `json:"processor_refund_id" db:"processor_refund_id"`
    CreatedAt     time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
    ProcessedAt   *time.Time      `json:"processed_at,omitempty" db:"processed_at"`
}

type RefundStatus string

const (
    RefundStatusPending   RefundStatus = "pending"
    RefundStatusProcessed RefundStatus = "processed"
    RefundStatusFailed    RefundStatus = "failed"
)
```

## API Design

### Endpoints

#### 1. Authorize Payment
```http
POST /api/payments/authorize
Content-Type: application/json
Authorization: Bearer <token>
X-Idempotency-Key: <unique-key>

{
  "order_id": "order-123",
  "customer_id": "customer-456",
  "amount": "99.99",
  "currency": "USD",
  "payment_method": {
    "type": "credit_card",
    "token": "pm_abc123",
    "last4": "4242"
  },
  "metadata": {
    "description": "Payment for order #123"
  }
}
```

Response:
```json
{
  "transaction_id": "txn_xyz789",
  "status": "authorized",
  "amount": "99.99",
  "currency": "USD",
  "auth_code": "AUTH123",
  "processor_name": "puppypay",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### 2. Capture Payment
```http
POST /api/payments/capture/{transactionId}
Content-Type: application/json
Authorization: Bearer <token>

{
  "amount": "99.99",
  "metadata": {
    "capture_reason": "order_shipped"
  }
}
```

Response:
```json
{
  "transaction_id": "txn_xyz789",
  "status": "captured",
  "captured_amount": "99.99",
  "captured_at": "2024-01-15T10:35:00Z"
}
```

#### 3. Process Refund
```http
POST /api/payments/refund/{transactionId}
Content-Type: application/json
Authorization: Bearer <token>

{
  "amount": "49.99",
  "reason": "partial_return",
  "metadata": {
    "return_items": ["item1", "item2"]
  }
}
```

Response:
```json
{
  "refund_id": "ref_abc456",
  "transaction_id": "txn_xyz789",
  "amount": "49.99",
  "status": "pending",
  "created_at": "2024-01-15T11:00:00Z"
}
```

#### 4. Get Payment Status
```http
GET /api/payments/status/{transactionId}
Authorization: Bearer <token>
```

Response:
```json
{
  "transaction_id": "txn_xyz789",
  "order_id": "order-123",
  "status": "captured",
  "amount": "99.99",
  "currency": "USD",
  "payment_method": "credit_card",
  "processor_name": "puppypay",
  "created_at": "2024-01-15T10:30:00Z",
  "captured_at": "2024-01-15T10:35:00Z",
  "refunds": [
    {
      "refund_id": "ref_abc456",
      "amount": "49.99",
      "status": "processed",
      "reason": "partial_return"
    }
  ]
}
```

#### 5. Webhook Handler
```http
POST /api/payments/webhook
Content-Type: application/json
X-Webhook-Signature: <signature>

{
  "event_type": "payment.captured",
  "transaction_id": "txn_xyz789",
  "processor_txn_id": "pi_stripe123",
  "timestamp": "2024-01-15T10:35:00Z",
  "data": {
    "amount": "99.99",
    "currency": "USD"
  }
}
```

### Error Responses

```json
{
  "error": {
    "code": "PAYMENT_DECLINED",
    "message": "Payment was declined by the processor",
    "details": {
      "processor": "kittycard",
      "decline_reason": "insufficient_funds"
    },
    "request_id": "req_123456",
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

## Mock Payment Processors

### Processor Interface
```go
type PaymentProcessor interface {
    // Authorize a payment without capturing funds
    Authorize(ctx context.Context, request *AuthorizeRequest) (*AuthorizeResponse, error)

    // Capture previously authorized payment
    Capture(ctx context.Context, request *CaptureRequest) (*CaptureResponse, error)

    // Process a refund
    Refund(ctx context.Context, request *RefundRequest) (*RefundResponse, error)

    // Get transaction status from processor
    GetStatus(ctx context.Context, processorTxnID string) (*StatusResponse, error)

    // Tokenize payment method for future use
    TokenizePaymentMethod(ctx context.Context, request *TokenizeRequest) (*TokenizeResponse, error)
}
```

### Mock Processor Implementations

#### PuppyPay (Primary Processor)
```go
type PuppyPayProcessor struct {
    config      PuppyPayConfig
    faultInjector *fault.Injector
    metrics     *telemetry.Metrics
}

// Simulates traditional credit card processing
// - Fast response times (50-200ms)
// - Standard auth/capture flow
// - Supports all major card types
// - Minimal failures in normal operation
```

#### KittyCard (Backup Processor)
```go
type KittyCardProcessor struct {
    config      KittyCardConfig
    faultInjector *fault.Injector
    metrics     *telemetry.Metrics
}

// Simulates alternative payment processor
// - Slightly slower response (100-300ms)
// - Higher failure rate (configurable, default 20%)
// - Different error codes and messages
// - Used for failover scenarios
```

#### DoggieCoin (Crypto Processor)
```go
type DoggieCoinProcessor struct {
    config      DoggieCoinConfig
    faultInjector *fault.Injector
    metrics     *telemetry.Metrics
}

// Simulates cryptocurrency payments
// - Very fast processing (10-50ms)
// - Different transaction flow (immediate settlement)
// - Blockchain simulation with confirmations
// - Unique error patterns (network congestion, etc.)
```

#### PawPal (Digital Wallet)
```go
type PawPalProcessor struct {
    config      PawPalConfig
    faultInjector *fault.Injector
    metrics     *telemetry.Metrics
}

// Simulates digital wallet payments
// - Medium response time (75-150ms)
// - OAuth-style authorization flow
// - Wallet balance checks
// - Account linking simulation
```

## Fault Injection Implementation

### Fault Configuration
```yaml
fault_injection:
  enabled: true
  payment:
    puppypay:
      failure_rate: 0.05          # 5% general failure rate
      latency_ms: 100             # Base latency
      latency_variation_ms: 50    # ±50ms variation
      failure_types:
        - type: "declined"
          rate: 0.03
          error_code: "card_declined"
        - type: "timeout"
          rate: 0.02
          latency_ms: 5000
      specific_failures:
        - match:
            card_number: "*4000"   # Test card numbers
          failure_rate: 1.0
          error_code: "always_decline"
        - match:
            amount: ">10000"       # High value transactions
          failure_rate: 0.1
          error_code: "fraud_check"

    kittycard:
      failure_rate: 0.2           # 20% failure rate
      latency_ms: 150
      failure_types:
        - type: "service_unavailable"
          rate: 0.15
          http_status: 503
        - type: "invalid_request"
          rate: 0.05
          error_code: "invalid_card"

    doggiecoin:
      failure_rate: 0.1
      latency_ms: 25
      failure_types:
        - type: "network_congestion"
          rate: 0.08
          latency_ms: 2000
        - type: "insufficient_balance"
          rate: 0.02
          error_code: "insufficient_crypto"

    pawpal:
      failure_rate: 0.08
      latency_ms: 100
      failure_types:
        - type: "account_locked"
          rate: 0.05
          error_code: "account_restricted"
        - type: "oauth_failure"
          rate: 0.03
          error_code: "invalid_token"
```

### Fault Injector Implementation
```go
type Injector struct {
    config FaultConfig
    rand   *rand.Rand
    mu     sync.RWMutex
}

type FaultResult struct {
    ShouldFail   bool
    ErrorType    string
    ErrorCode    string
    ErrorMessage string
    LatencyMs    int
    HTTPStatus   int
}

func (i *Injector) EvaluateRequest(ctx context.Context, processor string, request interface{}) *FaultResult {
    // 1. Check specific failure patterns first
    // 2. Apply general failure rate
    // 3. Add latency injection
    // 4. Return appropriate fault result
}
```

## Mock Database Implementation

### Memory Database Structure
```go
type MemoryDB struct {
    transactions    map[string]*domain.Transaction
    paymentMethods  map[string]*domain.PaymentMethod
    refunds         map[string]*domain.RefundRecord
    mu              sync.RWMutex
    persistence     *JSONPersistence
}

type JSONPersistence struct {
    dataDir     string
    enabled     bool
    saveInterval time.Duration
}

// Repository implementations
type TransactionRepository struct {
    db *MemoryDB
}

type PaymentMethodRepository struct {
    db *MemoryDB
}

type RefundRepository struct {
    db *MemoryDB
}
```

### Data Operations
```go
// Thread-safe CRUD operations
func (r *TransactionRepository) Create(ctx context.Context, txn *domain.Transaction) error
func (r *TransactionRepository) GetByID(ctx context.Context, id string) (*domain.Transaction, error)
func (r *TransactionRepository) Update(ctx context.Context, txn *domain.Transaction) error
func (r *TransactionRepository) GetByOrderID(ctx context.Context, orderID string) ([]*domain.Transaction, error)
func (r *TransactionRepository) GetByCustomerID(ctx context.Context, customerID string) ([]*domain.Transaction, error)
```

## OpenTelemetry Instrumentation

### Automatic Instrumentation Points

#### HTTP Middleware
```go
func TelemetryMiddleware(serviceName string) func(http.Handler) http.Handler {
    return otelhttp.NewMiddleware(serviceName,
        otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
        otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
            return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
        }),
    )
}
```

#### Database Operations
```go
func (r *TransactionRepository) Create(ctx context.Context, txn *domain.Transaction) error {
    ctx, span := otel.Tracer("payment-service").Start(ctx, "db.transaction.create")
    defer span.End()

    span.SetAttributes(
        attribute.String("db.operation", "create"),
        attribute.String("db.table", "transactions"),
        attribute.String("transaction.id", txn.ID),
    )

    // Actual database operation
    err := r.db.CreateTransaction(ctx, txn)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return err
}
```

#### Payment Processing
```go
func (s *PaymentService) AuthorizePayment(ctx context.Context, req *AuthorizeRequest) (*AuthorizeResponse, error) {
    ctx, span := otel.Tracer("payment-service").Start(ctx, "payment.authorize")
    defer span.End()

    span.SetAttributes(
        attribute.String("payment.processor", req.Processor),
        attribute.String("payment.method", string(req.PaymentMethod.Type)),
        attribute.String("payment.currency", req.Currency),
        attribute.Float64("payment.amount", req.Amount.InexactFloat64()),
    )

    // Business logic with nested spans
    // ...
}
```

### Custom Metrics
```go
var (
    PaymentRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "payment_requests_total",
            Help: "Total number of payment requests",
        },
        []string{"processor", "method", "status"},
    )

    PaymentDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "payment_duration_seconds",
            Help: "Payment processing duration",
            Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
        },
        []string{"processor", "operation"},
    )

    FaultInjectionTriggered = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "fault_injection_triggered_total",
            Help: "Number of times fault injection was triggered",
        },
        []string{"processor", "fault_type"},
    )
)
```

## Error Handling and Retry Strategies

### Error Classification
```go
type ErrorCategory string

const (
    ErrorCategoryTransient    ErrorCategory = "transient"    // Retry recommended
    ErrorCategoryPermanent    ErrorCategory = "permanent"    // Don't retry
    ErrorCategoryRateLimit    ErrorCategory = "rate_limit"   // Retry with backoff
    ErrorCategoryAuth         ErrorCategory = "auth"         // Check credentials
    ErrorCategoryBusiness     ErrorCategory = "business"     // Business logic error
)

type PaymentError struct {
    Code       string        `json:"code"`
    Message    string        `json:"message"`
    Category   ErrorCategory `json:"category"`
    Processor  string        `json:"processor,omitempty"`
    Retryable  bool          `json:"retryable"`
    RetryAfter time.Duration `json:"retry_after,omitempty"`
}
```

### Retry Logic
```go
type RetryConfig struct {
    MaxRetries      int           `yaml:"max_retries"`
    InitialBackoff  time.Duration `yaml:"initial_backoff"`
    MaxBackoff      time.Duration `yaml:"max_backoff"`
    BackoffMultiplier float64     `yaml:"backoff_multiplier"`
}

func (s *PaymentService) executeWithRetry(ctx context.Context, operation func() error) error {
    var lastErr error

    for attempt := 0; attempt <= s.retryConfig.MaxRetries; attempt++ {
        if attempt > 0 {
            backoff := s.calculateBackoff(attempt)
            select {
            case <-time.After(backoff):
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        err := operation()
        if err == nil {
            return nil
        }

        // Check if error is retryable
        if paymentErr, ok := err.(*PaymentError); ok && !paymentErr.Retryable {
            return err
        }

        lastErr = err
    }

    return fmt.Errorf("operation failed after %d attempts: %w", s.retryConfig.MaxRetries, lastErr)
}
```

### Circuit Breaker
```go
type CircuitBreaker struct {
    name           string
    maxFailures    int
    resetTimeout   time.Duration
    state          CircuitState
    failures       int
    lastFailTime   time.Time
    mu             sync.RWMutex
}

func (cb *CircuitBreaker) Execute(ctx context.Context, operation func() error) error {
    if !cb.allowRequest() {
        return ErrCircuitBreakerOpen
    }

    err := operation()
    cb.recordResult(err == nil)

    return err
}
```

## Configuration Structure

### Main Configuration
```yaml
app:
  name: "griffin-commerce-payment"
  environment: "poc"
  port: 8081
  log_level: "debug"
  shutdown_timeout: "30s"

database:
  mock_enabled: true
  persist_to_file: true
  data_dir: "./data"
  auto_save_interval: "30s"

payment:
  default_currency: "USD"
  max_amount: "10000.00"
  idempotency_ttl: "24h"

  processors:
    puppypay:
      enabled: true
      priority: 1
      timeout: "5s"
      max_retries: 3
    kittycard:
      enabled: true
      priority: 2
      timeout: "10s"
      max_retries: 2
    doggiecoin:
      enabled: true
      priority: 3
      timeout: "30s"
      max_retries: 1
    pawpal:
      enabled: true
      priority: 4
      timeout: "15s"
      max_retries: 2

security:
  rate_limit:
    requests_per_minute: 100
    burst_size: 20

  webhook:
    signature_verification: true
    max_body_size: "1MB"
    timeout: "10s"

telemetry:
  enabled: true
  service_name: "griffin-commerce-payment"
  otlp_endpoint: "localhost:4317"
  sampling_rate: 1.0

  metrics:
    enabled: true
    port: 9090
    path: "/metrics"

  traces:
    enabled: true
    batch_timeout: "5s"
    max_export_batch_size: 512

fault_injection:
  enabled: true
  config_file: "./configs/fault_injection.yaml"
  hot_reload: true
```

## Testing Strategy

### Unit Tests

#### Service Layer Tests
```go
func TestPaymentService_AuthorizePayment(t *testing.T) {
    tests := []struct {
        name           string
        request        *AuthorizeRequest
        processorResponse *AuthorizeResponse
        processorError error
        expectedStatus TransactionStatus
        expectedError  string
    }{
        {
            name: "successful_authorization",
            request: &AuthorizeRequest{
                Amount: decimal.NewFromFloat(99.99),
                Currency: "USD",
                // ... other fields
            },
            processorResponse: &AuthorizeResponse{
                TransactionID: "proc_123",
                AuthCode: "AUTH123",
                Status: "authorized",
            },
            expectedStatus: StatusAuthorized,
        },
        {
            name: "declined_payment",
            request: &AuthorizeRequest{
                Amount: decimal.NewFromFloat(99.99),
                Currency: "USD",
            },
            processorError: &PaymentError{
                Code: "card_declined",
                Category: ErrorCategoryPermanent,
            },
            expectedStatus: StatusFailed,
            expectedError: "card_declined",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

#### Processor Tests
```go
func TestPuppyPayProcessor_Authorize(t *testing.T) {
    tests := []struct {
        name          string
        faultConfig   *fault.Config
        request       *AuthorizeRequest
        expectedError string
        expectedLatency time.Duration
    }{
        {
            name: "no_faults",
            faultConfig: &fault.Config{Enabled: false},
            request: validAuthorizeRequest(),
        },
        {
            name: "timeout_injection",
            faultConfig: &fault.Config{
                Enabled: true,
                FailureRate: 1.0,
                FailureType: "timeout",
                LatencyMs: 5000,
            },
            expectedError: "timeout",
            expectedLatency: 5 * time.Second,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation with fault injection
        })
    }
}
```

### Integration Tests

#### End-to-End Payment Flow
```go
func TestPaymentFlow_AuthorizeAndCapture(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Close()

    client := payment.NewClient(server.URL)

    // Test complete payment flow
    authorizeResp, err := client.Authorize(context.Background(), &payment.AuthorizeRequest{
        OrderID: "test-order-123",
        Amount: decimal.NewFromFloat(99.99),
        Currency: "USD",
        PaymentMethod: &payment.PaymentMethod{
            Type: payment.TypeCreditCard,
            Token: "test-token",
        },
    })
    require.NoError(t, err)
    require.Equal(t, payment.StatusAuthorized, authorizeResp.Status)

    // Capture the payment
    captureResp, err := client.Capture(context.Background(), authorizeResp.TransactionID, &payment.CaptureRequest{
        Amount: decimal.NewFromFloat(99.99),
    })
    require.NoError(t, err)
    require.Equal(t, payment.StatusCaptured, captureResp.Status)
}
```

#### Fault Injection Tests
```go
func TestFaultInjection_ProcessorFailover(t *testing.T) {
    // Configure KittyCard to fail 100% of requests
    faultConfig := &fault.Config{
        Payment: map[string]fault.ProcessorConfig{
            "kittycard": {
                FailureRate: 1.0,
                FailureType: "service_unavailable",
            },
            "puppypay": {
                FailureRate: 0.0,
            },
        },
    }

    server := setupTestServerWithFaults(t, faultConfig)
    defer server.Close()

    client := payment.NewClient(server.URL)

    // Force request to try KittyCard first
    resp, err := client.Authorize(context.Background(), &payment.AuthorizeRequest{
        // ... request details
        PreferredProcessor: "kittycard",
    })

    // Should succeed with PuppyPay fallback
    require.NoError(t, err)
    require.Equal(t, "puppypay", resp.ProcessorName)
}
```

### Load Testing Scenarios
```go
func TestLoadHandling_ConcurrentPayments(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    client := payment.NewClient(server.URL)

    // Simulate 100 concurrent payment requests
    var wg sync.WaitGroup
    var successCount int64
    var errorCount int64

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()

            _, err := client.Authorize(context.Background(), &payment.AuthorizeRequest{
                OrderID: fmt.Sprintf("order-%d", i),
                Amount: decimal.NewFromFloat(float64(i + 1)),
                Currency: "USD",
                // ... other fields
            })

            if err != nil {
                atomic.AddInt64(&errorCount, 1)
            } else {
                atomic.AddInt64(&successCount, 1)
            }
        }(i)
    }

    wg.Wait()

    // Verify success rate
    successRate := float64(successCount) / 100.0
    require.Greater(t, successRate, 0.95, "Success rate should be > 95%")
}
```

## Implementation Order

### Phase 1: Core Infrastructure
1. Set up project structure and dependencies
2. Implement configuration management with YAML support
3. Create domain models and basic validation
4. Set up in-memory mock database with JSON persistence
5. Implement basic HTTP server with middleware
6. Add OpenTelemetry instrumentation setup

### Phase 2: Basic Payment Processing
1. Implement payment processor interface
2. Create PuppyPay mock processor (primary)
3. Implement transaction service with basic authorize/capture flow
4. Add transaction repository and basic CRUD operations
5. Create HTTP handlers for authorize and capture endpoints
6. Add basic error handling and logging

### Phase 3: Multiple Processors and Fault Injection
1. Implement KittyCard, DoggieCoin, and PawPal processors
2. Create processor factory with failover logic
3. Implement fault injection framework
4. Add processor-specific fault configurations
5. Create admin endpoints for fault management
6. Add comprehensive telemetry for all processors

### Phase 4: Advanced Features
1. Implement refund processing
2. Add webhook handling with signature verification
3. Create payment method tokenization
4. Implement rate limiting and security features
5. Add circuit breaker pattern
6. Create retry logic with exponential backoff

### Phase 5: Testing and Observability
1. Write comprehensive unit tests
2. Create integration test suite
3. Implement load testing scenarios
4. Add detailed monitoring and alerting
5. Create debug endpoints and health checks
6. Performance optimization and tuning

### Phase 6: Documentation and Deployment
1. Create API documentation
2. Write operational runbooks
3. Create deployment configurations
4. Set up monitoring dashboards
5. Conduct security review
6. Performance benchmarking

## Dependencies

### External Libraries
```go
// Core dependencies
"github.com/gorilla/mux"                    // HTTP routing
"github.com/shopspring/decimal"             // Precise decimal arithmetic
"go.opentelemetry.io/otel"                  // OpenTelemetry
"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Configuration and serialization
"gopkg.in/yaml.v3"                          // YAML configuration
"encoding/json"                             // JSON handling (stdlib)

// Testing
"github.com/stretchr/testify"              // Test assertions
"github.com/golang/mock"                    // Mock generation

// Utilities
"github.com/google/uuid"                    // UUID generation
"golang.org/x/time/rate"                   // Rate limiting
"golang.org/x/sync/errgroup"              // Error group handling
```

### Internal Dependencies
```
- Common Service: Shared logging, configuration, error handling
- No external databases (PostgreSQL, Redis) for POC
- No external payment processors (all mocked)
- No external monitoring systems (metrics exported via HTTP)
```

This comprehensive design provides a production-ready payment service that can be implemented incrementally, with strong emphasis on testability, observability, and fault tolerance. The mock-based approach allows for realistic testing scenarios while maintaining complete isolation from external dependencies.
