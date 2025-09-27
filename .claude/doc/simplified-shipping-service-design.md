# Simplified Shipping Service System Design

## Overview

The Simplified Shipping Service is a MINIMAL implementation designed for the Griffin Commerce 2-week POC. This service handles only basic shipping rate calculations and shipment submissions with random failures. All complex features have been removed to focus on core service communication patterns and fault tolerance demonstration.

## Architecture Principles

- **EXTREME SIMPLICITY**: Fixed rates, random failures only
- **No External Dependencies**: Uses Common Service utilities only
- **Fast Implementation**: Half-day implementation target
- **Observable**: Basic OpenTelemetry integration via Common Service
- **Testable**: Simple unit tests for rate calculation and failure simulation

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                Simplified Shipping Service             │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────────────────────┐ │
│  │  Rate Calculator│  │    Shipment Submitter          │ │
│  │  (Fixed Rates)  │  │  (Random Failures Only)        │ │
│  └─────────────────┘  └─────────────────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│                    Mock Carriers (3)                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │
│  │PonyExpress  │ │  AvianAir   │ │   CatCarrier        │ │
│  │ $9.99, 5%   │ │ $19.99, 10% │ │  $14.99, 25%        │ │
│  │  failure    │ │   failure   │ │   failure           │ │
│  └─────────────┘ └─────────────┘ └─────────────────────┘ │
├─────────────────────────────────────────────────────────┤
│              Common Service Integration                 │
│     (Telemetry, Logging, HTTP, Config, Error)          │
└─────────────────────────────────────────────────────────┘
```

## File Structure (FLAT Design)

```
/shipping/
├── main.go           # Server setup and startup
├── handlers.go       # HTTP request handlers
├── carriers.go       # Mock carrier logic (3 carriers)
├── config.yaml       # Carrier configuration
├── models.go         # Simple data models
└── handlers_test.go  # Basic unit tests
```

## Core Component Specifications

### 1. Main Server Setup (`main.go`)

**Responsibilities**:
- Initialize Common Service utilities
- Load carrier configuration from YAML
- Set up HTTP routes
- Start server on port 8084

**Key Components**:
```go
func main() {
    // Load configuration
    cfg := loadConfig("config.yaml")

    // Initialize Common Service dependencies
    logger := common.NewLogger()
    telemetry := common.NewTelemetry()

    // Initialize carriers
    carrierManager := NewCarrierManager(cfg.Carriers)

    // Set up handlers
    handlers := NewHandlers(carrierManager, logger)

    // Configure routes
    router := mux.NewRouter()
    router.HandleFunc("/api/shipping/calculate", handlers.CalculateRates).Methods("POST")
    router.HandleFunc("/api/shipping/submit", handlers.SubmitShipment).Methods("POST")

    // Start server
    server := &http.Server{
        Addr:    ":8084",
        Handler: router,
    }

    log.Fatal(server.ListenAndServe())
}
```

### 2. HTTP Handlers (`handlers.go`)

**Responsibilities**:
- Handle rate calculation requests
- Handle shipment submission requests
- Return appropriate HTTP responses
- Log requests using Common Service

**CalculateRates Handler**:
```go
func (h *Handlers) CalculateRates(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Parse request (minimal validation)
    var req CalculateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Get rates from all carriers (always returns fixed rates)
    rates := h.carrierManager.GetAllRates()

    // Return response
    response := CalculateResponse{Rates: rates}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)

    h.logger.Info(ctx, "Rate calculation completed",
        common.Field{Key: "carriers_count", Value: len(rates)})
}
```

**SubmitShipment Handler**:
```go
func (h *Handlers) SubmitShipment(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Parse request
    var req SubmitRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Submit to carrier (may fail randomly)
    shipment, err := h.carrierManager.SubmitShipment(ctx, req.OrderID, req.Carrier)
    if err != nil {
        h.logger.Error(ctx, "Shipment submission failed",
            common.Field{Key: "carrier", Value: req.Carrier},
            common.Field{Key: "error", Value: err.Error()})

        http.Error(w, "Shipment failed", http.StatusServiceUnavailable)
        return
    }

    // Return success response
    response := SubmitResponse{Shipment: shipment}
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)

    h.logger.Info(ctx, "Shipment submitted successfully",
        common.Field{Key: "shipment_id", Value: shipment.ID},
        common.Field{Key: "carrier", Value: req.Carrier})
}
```

### 3. Mock Carrier Logic (`carriers.go`)

**Responsibilities**:
- Store carrier configurations (rates and failure rates)
- Return fixed rates for all carriers
- Simulate random failures based on carrier failure rates
- Generate simple shipment records

**CarrierManager Implementation**:
```go
type CarrierManager struct {
    carriers map[string]*Carrier
    random   *rand.Rand
}

type Carrier struct {
    ID          string  `yaml:"id"`
    Name        string  `yaml:"name"`
    Rate        float64 `yaml:"rate"`
    FailureRate float64 `yaml:"failure_rate"`
}

func NewCarrierManager(config map[string]*Carrier) *CarrierManager {
    return &CarrierManager{
        carriers: config,
        random:   rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (cm *CarrierManager) GetAllRates() []Rate {
    var rates []Rate
    for id, carrier := range cm.carriers {
        rates = append(rates, Rate{
            Carrier: id,
            Name:    carrier.Name,
            Cost:    carrier.Rate,
        })
    }
    return rates
}

func (cm *CarrierManager) SubmitShipment(ctx context.Context, orderID, carrierID string) (*Shipment, error) {
    carrier, exists := cm.carriers[carrierID]
    if !exists {
        return nil, fmt.Errorf("unknown carrier: %s", carrierID)
    }

    // Random failure based on carrier's failure rate
    if cm.random.Float64() < carrier.FailureRate {
        return nil, fmt.Errorf("carrier %s submission failed", carrierID)
    }

    // Success - create shipment
    shipment := &Shipment{
        ID:      fmt.Sprintf("SHIP-%d", time.Now().Unix()),
        OrderID: orderID,
        Carrier: carrierID,
        Status:  "submitted",
        Cost:    carrier.Rate,
    }

    return shipment, nil
}
```

### 4. Data Models (`models.go`)

**Simple Data Structures**:
```go
// Shipment represents a minimal shipment
type Shipment struct {
    ID      string  `json:"id"`
    OrderID string  `json:"order_id"`
    Carrier string  `json:"carrier"`
    Status  string  `json:"status"` // "submitted" or "failed"
    Cost    float64 `json:"cost"`
}

// Rate represents a shipping rate option
type Rate struct {
    Carrier string  `json:"carrier"`
    Name    string  `json:"name"`
    Cost    float64 `json:"cost"`
}

// CalculateRequest for rate calculation
type CalculateRequest struct {
    OrderID string `json:"order_id"` // Not used, just for logging
}

// CalculateResponse with all available rates
type CalculateResponse struct {
    Rates []Rate `json:"rates"`
}

// SubmitRequest for shipment submission
type SubmitRequest struct {
    OrderID string `json:"order_id"`
    Carrier string `json:"carrier"`
}

// SubmitResponse for successful submission
type SubmitResponse struct {
    Shipment *Shipment `json:"shipment"`
}
```

## Configuration File (`config.yaml`)

**Carrier Configuration**:
```yaml
# Simplified Shipping Service Configuration
service:
  name: "shipping-service"
  port: 8084

carriers:
  ponyexpress:
    name: "Pony Express Ground"
    rate: 9.99
    failure_rate: 0.05  # 5% failure rate

  avianair:
    name: "Avian Air Express"
    rate: 19.99
    failure_rate: 0.10  # 10% failure rate

  catcarrier:
    name: "Cat Carrier Delivery"
    rate: 14.99
    failure_rate: 0.25  # 25% failure rate (cats are unpredictable!)

telemetry:
  enabled: true
  service_name: "shipping-service"

logging:
  level: "info"
```

## API Specifications

### Calculate Shipping Rates

**Endpoint**: `POST /api/shipping/calculate`

**Request**:
```json
{
  "order_id": "ORDER-123"
}
```

**Response**:
```json
{
  "rates": [
    {
      "carrier": "ponyexpress",
      "name": "Pony Express Ground",
      "cost": 9.99
    },
    {
      "carrier": "avianair",
      "name": "Avian Air Express",
      "cost": 19.99
    },
    {
      "carrier": "catcarrier",
      "name": "Cat Carrier Delivery",
      "cost": 14.99
    }
  ]
}
```

### Submit Shipment

**Endpoint**: `POST /api/shipping/submit`

**Request**:
```json
{
  "order_id": "ORDER-123",
  "carrier": "ponyexpress"
}
```

**Success Response** (HTTP 200):
```json
{
  "shipment": {
    "id": "SHIP-1674567890",
    "order_id": "ORDER-123",
    "carrier": "ponyexpress",
    "status": "submitted",
    "cost": 9.99
  }
}
```

**Failure Response** (HTTP 503):
```json
{
  "error": "Shipment failed"
}
```

## Integration with Common Service

### Dependencies Used

**HTTP Middleware Stack**:
- Request logging with correlation IDs
- OpenTelemetry tracing for request spans
- Error handling and recovery
- CORS support for web requests

**Configuration Management**:
- YAML configuration loading via Common Service
- Environment variable substitution
- Configuration validation

**Logging Integration**:
- Structured logging with trace IDs
- Context-aware logging with request metadata
- Log level configuration

**Telemetry Integration**:
```go
// Automatic HTTP instrumentation via Common Service middleware
func (h *Handlers) CalculateRates(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Span automatically created by Common Service HTTP middleware
    // Manual span creation for carrier operations
    ctx, span := h.telemetry.CreateSpan(ctx, "shipping.calculate_rates")
    defer span.End()

    span.SetAttributes(
        attribute.Int("carriers.count", len(h.carrierManager.carriers)),
    )

    // Business logic...
}
```

## Testing Strategy

### Unit Testing Requirements (`handlers_test.go`)

**Rate Calculation Tests**:
```go
func TestHandlers_CalculateRates(t *testing.T) {
    // Setup test carriers
    carriers := map[string]*Carrier{
        "test_carrier": {
            ID:   "test_carrier",
            Name: "Test Carrier",
            Rate: 10.00,
            FailureRate: 0.0,
        },
    }

    manager := NewCarrierManager(carriers)
    handlers := NewHandlers(manager, mockLogger)

    // Test rate calculation
    req := httptest.NewRequest("POST", "/api/shipping/calculate",
        strings.NewReader(`{"order_id":"TEST-123"}`))
    w := httptest.NewRecorder()

    handlers.CalculateRates(w, req)

    assert.Equal(t, http.StatusOK, w.Code)

    var response CalculateResponse
    json.NewDecoder(w.Body).Decode(&response)

    assert.Len(t, response.Rates, 1)
    assert.Equal(t, "test_carrier", response.Rates[0].Carrier)
    assert.Equal(t, 10.00, response.Rates[0].Cost)
}
```

**Shipment Submission Tests**:
```go
func TestHandlers_SubmitShipment_Success(t *testing.T) {
    // Test successful submission (0% failure rate)
}

func TestHandlers_SubmitShipment_Failure(t *testing.T) {
    // Test failure simulation (100% failure rate)
}

func TestCarrierManager_FailureRate(t *testing.T) {
    // Test that failure rates are approximately correct over many attempts
    carrier := &Carrier{FailureRate: 0.5}
    manager := NewCarrierManager(map[string]*Carrier{"test": carrier})

    failures := 0
    attempts := 1000

    for i := 0; i < attempts; i++ {
        _, err := manager.SubmitShipment(context.Background(), "test-order", "test")
        if err != nil {
            failures++
        }
    }

    // Should be approximately 50% ± tolerance
    assert.InDelta(t, 500, failures, 50)
}
```

### Integration Testing Requirements

**End-to-End Flow Tests**:
1. Start service with test configuration
2. Calculate rates via HTTP API
3. Submit shipment via HTTP API
4. Verify telemetry spans are created
5. Verify logs contain expected correlation IDs

**Service Communication Tests**:
- Test integration with Common Service middleware
- Verify OpenTelemetry context propagation
- Test error response formatting via Common Service

## Implementation Order

### Phase 1: Core Implementation (4 hours)
1. Create basic file structure
2. Implement carrier configuration loading
3. Implement rate calculation (return fixed rates)
4. Implement shipment submission with random failures
5. Basic HTTP handlers

### Phase 2: Integration & Testing (2 hours)
1. Integrate with Common Service middleware
2. Add basic unit tests
3. Test HTTP endpoints manually
4. Verify telemetry integration

### Phase 3: Polish (1 hour)
1. Add error handling improvements
2. Validate configuration loading
3. Final testing and documentation updates

## Dependencies

### External Libraries (via Common Service)
```go
// HTTP routing (Common Service dependency)
"github.com/gorilla/mux"

// Configuration (Common Service dependency)
"gopkg.in/yaml.v3"

// OpenTelemetry (Common Service dependency)
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/attribute"

// Testing
"github.com/stretchr/testify"
```

### Internal Dependencies
- Common Service package (all utilities)
- No dependencies on other microservices
- Self-contained with minimal external calls

## Performance Requirements

### Response Time Targets
- Rate calculation: < 10ms (returns fixed data)
- Shipment submission: < 50ms (includes random delay simulation)
- Service startup: < 2 seconds

### Throughput Targets
- Handle 1,000+ requests/second for rate calculations
- Handle 500+ requests/second for shipment submissions
- Minimal memory footprint (< 50MB)

## Monitoring & Health Checks

### Health Endpoints (via Common Service)
- `/health` - Basic service health
- `/ready` - Readiness probe
- `/metrics` - Prometheus metrics

### Key Metrics to Monitor
- `shipping_rate_calculations_total` - Rate calculation requests
- `shipping_submissions_total{status="success|failed"}` - Submission attempts
- `shipping_carrier_failures_total{carrier}` - Failures by carrier
- Standard HTTP metrics via Common Service middleware

### Alerting Scenarios
- Shipment submission failure rate > 50% (indicates configuration issue)
- Service not responding to health checks
- Error rate > 10% for rate calculations (should never fail)

## Fault Tolerance Demonstration

### Failure Scenarios Simulated
1. **PonyExpress (5% failure)**: Occasional reliable carrier failures
2. **AvianAir (10% failure)**: Moderate failure rate for testing retry logic
3. **CatCarrier (25% failure)**: High failure rate showing resilience patterns

### Client Retry Recommendations
- Rate calculations: No retries needed (should always succeed)
- Shipment submissions: Implement exponential backoff
- Consider trying different carriers on failure
- Maximum 3 retry attempts per carrier

## What This Design Achieves

### POC Objectives Met
1. **Service Communication**: HTTP API calls between microservices
2. **Fault Tolerance**: Random failure simulation for resilience testing
3. **Observability**: Request tracing and logging via Common Service
4. **Configuration**: YAML-based carrier configuration
5. **Simplicity**: Minimal implementation focused on core patterns

### Removed Complexities
- ❌ Label generation
- ❌ Package optimization
- ❌ Address validation
- ❌ Tracking updates
- ❌ Zone-based pricing
- ❌ Multi-package shipments
- ❌ Weight/dimension calculations
- ❌ Complex carrier integrations

### Implementation Reality
- **Total implementation time**: ~7 hours for a complete working service
- **Lines of code**: ~500 lines (excluding tests)
- **External dependencies**: Only Common Service utilities
- **Maintenance**: Minimal - just carrier configuration updates

This simplified design provides exactly what's needed for the Griffin Commerce POC: a working shipping service that demonstrates microservice communication patterns, fault tolerance through random failures, and observability integration, while being implementable in half a day.