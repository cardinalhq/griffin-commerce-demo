# POC Architecture Requirements

## Overview
This document outlines the POC-specific architectural decisions and requirements for the Griffin Commerce demo, focusing on simplicity, observability, and testability without external dependencies.

## Mock Database Layer

### Requirements
- Each service maintains its own in-memory mock database
- Data persistence to JSON files for restart recovery (optional)
- Thread-safe operations using sync.RWMutex
- Simple CRUD interface matching future real database patterns
- Configurable initial data loading from YAML

### Implementation Pattern
```go
package mockdb

type MockDB interface {
    Create(ctx context.Context, entity interface{}) error
    Read(ctx context.Context, id string, entity interface{}) error
    Update(ctx context.Context, id string, entity interface{}) error
    Delete(ctx context.Context, id string) error
    List(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)
}
```

## External Service Simulation

### Mock External Services
- Payment Processors (PuppyPay, KittyCard, DoggieCoin, PawPal)
- Shipping Carriers (PonyExpress, CatCarrier, AvianAir, TurtleTransit)
- Email Service (HootMail)
- SMS Service (ChirpText)
- Image CDN (FurryFast CDN)
- Address Validation Service (NestFinder)

### Implementation
- Each external service has a mock client implementing the same interface as real client
- Configurable response delays to simulate network latency
- Predefined response scenarios (success, various failures)
- Request/response logging for debugging

## Fault Injection Framework

### Fault Types
- Service failures (connection refused, timeout)
- Partial failures (slow response, intermittent errors)
- Data corruption scenarios
- Rate limiting simulation
- Network partition simulation

### Configuration
```yaml
fault_injection:
  payment:
    puppypay:
      failure_rate: 0.0  # 0-1, percentage of requests to fail
      failure_type: "timeout"  # timeout, error, partial
      latency_ms: 100
      specific_failures:
        - match:
            card_number: "*4242"
          failure_rate: 0.2
          error: "insufficient_funds"
    kittycard:
      failure_rate: 0.2  # 20% failure rate for KittyCard
      failure_type: "declined"
      latency_ms: 150
    doggiecoin:
      failure_rate: 0.0
      latency_ms: 50  # Fast crypto!
  shipping:
    ponyexpress:
      failure_rate: 0.05
      failure_type: "service_unavailable"
    catcarrier:
      failure_rate: 0.1  # Cats are unpredictable
      failure_type: "timeout"
```

### Implementation
```go
type FaultInjector struct {
    ServiceName string
    Config      FaultConfig
}

func (f *FaultInjector) ShouldFail(request interface{}) (bool, error) {
    // Check specific failure rules first
    // Then check general failure rate
    // Return appropriate error if should fail
}
```

## YAML Configuration Structure

### Main Configuration File (`config.yaml`)
```yaml
app:
  name: "Griffin Commerce Demo"
  environment: "poc"
  port: 8080
  log_level: "debug"

telemetry:
  enabled: true
  service_name: "griffin-commerce"
  otlp_endpoint: "localhost:4317"
  sampling_rate: 1.0  # 100% sampling for POC

services:
  payment:
    port: 8081
    mock_mode: true
    providers:
      - name: "puppypay"
        enabled: true
      - name: "kittycard"
        enabled: true
      - name: "doggiecoin"
        enabled: true
      - name: "pawpal"
        enabled: true

  cart:
    port: 8082
    mock_mode: true
    session_timeout_minutes: 30
    max_items: 50

  images:
    port: 8083
    mock_mode: true
    storage_path: "./mock-images"
    cdn_url: "http://localhost:8083/images"

  shipping:
    port: 8084
    mock_mode: true
    carriers:
      - name: "ponyexpress"
        enabled: true
      - name: "catcarrier"
        enabled: true
      - name: "avianair"
        enabled: true
      - name: "turtletransit"
        enabled: true

  recommendations:
    port: 8085
    mock_mode: true
    algorithm: "simple"  # simple, collaborative, hybrid

  common:
    database:
      mock: true
      persist_to_file: true
      data_dir: "./mock-data"
    cache:
      mock: true
      ttl_seconds: 300
```

### Product Catalog File (`products.yaml`)
```yaml
products:
  - id: "DOG-TOY-001"
    sku: "ROPE-TOY-LG"
    name: "Premium Rope Toy - Large"
    description: "Durable cotton rope toy for large dogs"
    category: "toys"
    price: 15.99
    currency: "USD"
    weight_oz: 8
    dimensions:
      length: 12
      width: 3
      height: 3
    images:
      - url: "rope-toy-lg-1.jpg"
        alt: "Large rope toy main view"
      - url: "rope-toy-lg-2.jpg"
        alt: "Dog playing with rope toy"
    inventory: 100
    tags: ["large-dogs", "durable", "interactive"]

  - id: "DOG-TREAT-001"
    sku: "BACON-TREATS-SM"
    name: "Bacon Flavored Training Treats"
    description: "Small training treats with real bacon flavor"
    category: "treats"
    price: 9.99
    currency: "USD"
    weight_oz: 4
    images:
      - url: "bacon-treats-1.jpg"
        alt: "Bacon treats package"
    inventory: 250
    tags: ["training", "small-bites", "bacon-flavor"]

  - id: "DOG-FOOD-001"
    sku: "PUPPY-FOOD-GRAIN-FREE"
    name: "Grain-Free Puppy Food - 15lb"
    description: "Premium grain-free formula for puppies"
    category: "food"
    price: 45.99
    currency: "USD"
    weight_oz: 240
    images:
      - url: "puppy-food-1.jpg"
        alt: "Grain-free puppy food bag"
    inventory: 50
    tags: ["puppy", "grain-free", "premium"]
```

## OpenTelemetry Implementation

### Automatic Instrumentation
- HTTP middleware for all services (latency, status codes, request/response size)
- Database operation spans (even for mock DB)
- External service call tracing
- Automatic error recording and stack traces
- Context propagation across service boundaries

### Manual Instrumentation Points
- Business metrics (cart value, conversion rate, etc.)
- Custom spans for complex operations
- Feature flag evaluations
- A/B test assignments
- User journey tracking

### Metrics to Collect
```go
// Automatic HTTP metrics
- http_request_duration_seconds
- http_request_size_bytes
- http_response_size_bytes
- http_requests_total

// Business metrics
- cart_value_dollars
- cart_items_count
- checkout_conversion_rate
- payment_success_rate
- recommendation_click_through_rate
- fault_injection_triggered_total
```

### Trace Structure
```
[Frontend Request]
  └─[API Gateway]
      ├─[Cart Service]
      │   ├─[Mock DB Read]
      │   └─[Price Calculation]
      ├─[Recommendations Service]
      │   ├─[User Profile Lookup]
      │   └─[Algorithm Execution]
      └─[Image Service]
          └─[CDN Simulation]
```

### Logging Standards
```go
// Structured logging with context
logger.InfoContext(ctx, "Processing payment",
    "order_id", orderID,
    "amount", amount,
    "currency", currency,
    "processor", "stripe",
    "trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID(),
)
```

### Configuration
```go
// OTLP exporter setup
func InitTelemetry(config TelemetryConfig) (*trace.TracerProvider, error) {
    // Create OTLP trace exporter
    exporter, err := otlptrace.New(
        context.Background(),
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(config.Endpoint),
            otlptracegrpc.WithInsecure(),
        ),
    )

    // Create tracer provider with sampling
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithSampler(trace.TraceIDRatioBased(config.SamplingRate)),
        trace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String(config.ServiceName),
        )),
    )

    return tp, nil
}
```

## Development & Testing Tools

### Mock Data Generation
- Faker library for realistic test data
- Configurable data scenarios (normal, peak, edge cases)
- Bulk data generation for load testing

### Health & Debug Endpoints
- `/debug/config` - Current configuration (sanitized)
- `/debug/mock-db` - Mock database contents
- `/debug/faults` - Active fault injection rules
- `/metrics` - Prometheus metrics endpoint
- `/health` - Service health check
- `/ready` - Readiness probe

### Testing Helpers
```go
// Test scenario loader
func LoadTestScenario(name string) (*TestData, error)

// Fault injection helper for tests
func WithFaultInjection(rate float64, errorType string) TestOption

// Telemetry assertion helpers
func AssertSpanCreated(t *testing.T, operationName string)
func AssertMetricRecorded(t *testing.T, metricName string)
```

## Notes
- All external dependencies are mocked for true isolation
- Configuration is hot-reloadable without service restart
- Fault injection is controllable via API for dynamic testing
- OpenTelemetry data can be exported to Jaeger/Prometheus for local development
- Mock databases can optionally persist to JSON for data preservation across restarts