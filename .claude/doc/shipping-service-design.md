# Shipping Service System Design

## Overview

The Shipping Service is a core component of the Griffin Commerce demo that manages all shipping-related operations including rate calculations, label generation, package optimization, tracking, and carrier integrations. This service is designed as a POC with mock carrier implementations and full observability support.

## System Architecture

### High-Level Components

```
┌─────────────────────────────────────────────────────────────┐
│                    Shipping Service                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐  │
│  │   Rate Engine   │  │  Label Engine   │  │ Track Engine│  │
│  └─────────────────┘  └─────────────────┘  └─────────────┘  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐  │
│  │ Package Manager │  │ Address Service │  │ Notification│  │
│  └─────────────────┘  └─────────────────┘  └─────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                 Carrier Abstraction Layer                   │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────┐ │
│  │ PonyExpress │ │ CatCarrier  │ │  AvianAir   │ │ Turtle │ │
│  │    Mock     │ │    Mock     │ │    Mock     │ │Transit │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────┘ │
├─────────────────────────────────────────────────────────────┤
│                 Fault Injection Layer                       │
├─────────────────────────────────────────────────────────────┤
│               In-Memory Mock Database                       │
└─────────────────────────────────────────────────────────────┘
```

## File Structure

```
/pkg/shipping/
├── cmd/
│   └── main.go                     # Service entry point
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── rates.go            # Rate calculation endpoints
│   │   │   ├── shipment.go         # Shipment creation/management
│   │   │   ├── tracking.go         # Tracking endpoints
│   │   │   ├── address.go          # Address validation
│   │   │   └── labels.go           # Label generation
│   │   ├── middleware/
│   │   │   ├── telemetry.go        # OpenTelemetry middleware
│   │   │   ├── auth.go             # Authentication
│   │   │   └── ratelimit.go        # Rate limiting
│   │   └── server.go               # HTTP server setup
│   ├── carriers/
│   │   ├── interface.go            # Carrier interface definition
│   │   ├── ponyexpress/
│   │   │   ├── client.go           # PonyExpress mock client
│   │   │   └── models.go           # PonyExpress-specific models
│   │   ├── catcarrier/
│   │   │   ├── client.go           # CatCarrier mock client
│   │   │   └── models.go           # CatCarrier-specific models
│   │   ├── avianair/
│   │   │   ├── client.go           # AvianAir mock client
│   │   │   └── models.go           # AvianAir-specific models
│   │   ├── turtletransit/
│   │   │   ├── client.go           # TurtleTransit mock client
│   │   │   └── models.go           # TurtleTransit-specific models
│   │   ├── factory.go              # Carrier factory
│   │   └── manager.go              # Carrier management
│   ├── services/
│   │   ├── rates/
│   │   │   ├── engine.go           # Rate calculation engine
│   │   │   ├── calculator.go       # Core calculation logic
│   │   │   ├── zones.go            # Zone-based pricing
│   │   │   └── promotions.go       # Discount/promotion logic
│   │   ├── packages/
│   │   │   ├── optimizer.go        # Package optimization
│   │   │   ├── selector.go         # Box selection algorithm
│   │   │   └── consolidator.go     # Multi-package consolidation
│   │   ├── labels/
│   │   │   ├── generator.go        # Label generation
│   │   │   ├── formatter.go        # Label formatting (4x6, 8.5x11)
│   │   │   └── qrcode.go           # QR code generation
│   │   ├── tracking/
│   │   │   ├── simulator.go        # Tracking event simulation
│   │   │   ├── webhook.go          # Webhook processing
│   │   │   └── estimator.go        # Delivery estimation
│   │   ├── address/
│   │   │   ├── validator.go        # Address validation
│   │   │   ├── standardizer.go     # Address standardization
│   │   │   └── geocoder.go         # Geocoding simulation
│   │   └── notifications/
│   │       ├── sender.go           # Notification sender
│   │       ├── templates.go        # Email/SMS templates
│   │       └── events.go           # Event definitions
│   ├── models/
│   │   ├── shipment.go             # Shipment models
│   │   ├── address.go              # Address models
│   │   ├── package.go              # Package models
│   │   ├── rate.go                 # Rate models
│   │   ├── tracking.go             # Tracking models
│   │   └── label.go                # Label models
│   ├── repository/
│   │   ├── interface.go            # Repository interfaces
│   │   ├── memory/
│   │   │   ├── shipment.go         # In-memory shipment repo
│   │   │   ├── tracking.go         # In-memory tracking repo
│   │   │   └── address.go          # In-memory address repo
│   │   └── mock.go                 # Mock database implementation
│   ├── fault/
│   │   ├── injector.go             # Fault injection framework
│   │   ├── config.go               # Fault configuration
│   │   └── scenarios.go            # Predefined fault scenarios
│   └── config/
│       ├── config.go               # Configuration structure
│       ├── carriers.go             # Carrier configurations
│       └── validation.go           # Config validation
├── pkg/
│   ├── telemetry/
│   │   ├── tracer.go               # OpenTelemetry setup
│   │   ├── metrics.go              # Custom metrics
│   │   └── logger.go               # Structured logging
│   └── utils/
│       ├── weights.go              # Weight/dimension utilities
│       ├── zones.go                # Zone calculation utilities
│       └── time.go                 # Time calculation utilities
├── configs/
│   ├── carriers.yaml               # Carrier configuration
│   ├── zones.yaml                  # Shipping zones
│   ├── packages.yaml               # Package types
│   └── fault-injection.yaml        # Fault injection rules
├── test/
│   ├── integration/
│   │   ├── carriers_test.go        # Carrier integration tests
│   │   ├── rates_test.go           # Rate calculation tests
│   │   └── tracking_test.go        # Tracking simulation tests
│   ├── unit/
│   │   ├── services/               # Unit tests for services
│   │   ├── carriers/               # Unit tests for carriers
│   │   └── handlers/               # Unit tests for handlers
│   └── fixtures/
│       ├── addresses.yaml          # Test address data
│       ├── shipments.yaml          # Test shipment data
│       └── packages.yaml           # Test package data
└── Dockerfile                      # Container definition
```

## Core Component Specifications

### 1. Rate Calculation Engine

**Location**: `/pkg/shipping/internal/services/rates/`

**Responsibilities**:
- Calculate shipping rates from multiple carriers
- Apply zone-based pricing
- Handle promotions and free shipping thresholds
- Implement rate shopping across carriers
- Cache rates for 24 hours

**Key Components**:

**RateEngine** (`engine.go`):
```go
type RateEngine struct {
    carriers    []carriers.Carrier
    calculator  *Calculator
    cache       cache.Cache
    config      *config.RatesConfig
}

func (e *RateEngine) CalculateRates(ctx context.Context, req *RateRequest) (*RateResponse, error)
func (e *RateEngine) GetBestRate(ctx context.Context, req *RateRequest) (*Rate, error)
```

**Calculator** (`calculator.go`):
```go
type Calculator struct {
    zoneCalculator *ZoneCalculator
    promoEngine    *PromotionEngine
}

func (c *Calculator) Calculate(ctx context.Context, pkg *Package, addr *Address, service string) (*Rate, error)
```

**ZoneCalculator** (`zones.go`):
```go
type ZoneCalculator struct {
    zones map[string]*Zone
}

func (z *ZoneCalculator) GetZone(origin, destination *Address) (*Zone, error)
func (z *ZoneCalculator) CalculateZoneRate(zone *Zone, weight float64, service string) (float64, error)
```

### 2. Mock Carrier Implementation

**Location**: `/pkg/shipping/internal/carriers/`

Each carrier will have unique characteristics and behaviors:

**PonyExpress** (Fast and reliable):
- Services: Standard Trot (2-3 days), Priority Gallop (1-2 days), Express Sprint (overnight)
- Characteristics: 95% success rate, fast response times, premium pricing
- Special behavior: Extra fast in rural areas

**CatCarrier** (Unpredictable):
- Services: Prowl Delivery (3-5 days), Pounce Express (1-2 days), Midnight Dash (overnight)
- Characteristics: 85% success rate, random delays, mood-dependent pricing
- Special behavior: 20% chance of "nap delays", occasionally refuses certain packages

**AvianAir** (Eco-friendly):
- Services: Ground Waddle (4-6 days), Sky Glide (2-3 days), Eagle Express (overnight)
- Characteristics: 90% success rate, carbon-neutral options, weather-dependent
- Special behavior: Delays during migration seasons, premium for fragile items

**TurtleTransit** (Slow but reliable):
- Services: Economy Shell (7-10 days)
- Characteristics: 99% success rate, slowest but cheapest, handles heavy items
- Special behavior: Never fails, but always takes the maximum estimated time

**Carrier Interface** (`interface.go`):
```go
type Carrier interface {
    GetRates(ctx context.Context, req *RateRequest) ([]*Rate, error)
    CreateShipment(ctx context.Context, req *ShipmentRequest) (*Shipment, error)
    GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error)
    CancelShipment(ctx context.Context, shipmentID string) error
    GenerateLabel(ctx context.Context, shipmentID string) (*Label, error)
    ValidateAddress(ctx context.Context, addr *Address) (*AddressValidation, error)
}
```

### 3. Package Optimization System

**Location**: `/pkg/shipping/internal/services/packages/`

**PackageOptimizer** (`optimizer.go`):
```go
type PackageOptimizer struct {
    boxSelector    *BoxSelector
    consolidator   *Consolidator
    rules          []*PackagingRule
}

func (o *PackageOptimizer) OptimizePackaging(ctx context.Context, items []*Item) ([]*Package, error)
func (o *PackageOptimizer) CalculateDimensions(items []*Item) (*Dimensions, error)
```

**BoxSelector** (`selector.go`):
```go
type BoxSelector struct {
    availableBoxes []*Box
    rules          []*BoxRule
}

func (s *BoxSelector) SelectOptimalBox(items []*Item, requirements *Requirements) (*Box, error)
```

### 4. Label Generation System

**Location**: `/pkg/shipping/internal/services/labels/`

**LabelGenerator** (`generator.go`):
```go
type LabelGenerator struct {
    formatter  *LabelFormatter
    qrGenerator *QRCodeGenerator
    templates  map[string]*LabelTemplate
}

func (g *LabelGenerator) GenerateLabel(ctx context.Context, shipment *Shipment) (*Label, error)
func (g *LabelGenerator) GenerateReturnLabel(ctx context.Context, shipment *Shipment) (*Label, error)
func (g *LabelGenerator) GeneratePackingSlip(ctx context.Context, shipment *Shipment) (*PackingSlip, error)
```

### 5. Tracking Simulation Engine

**Location**: `/pkg/shipping/internal/services/tracking/`

**TrackingSimulator** (`simulator.go`):
```go
type TrackingSimulator struct {
    eventGenerator *EventGenerator
    estimator      *DeliveryEstimator
    notifier       *NotificationSender
}

func (s *TrackingSimulator) StartTracking(ctx context.Context, shipment *Shipment) error
func (s *TrackingSimulator) GenerateNextEvent(ctx context.Context, shipment *Shipment) (*TrackingEvent, error)
func (s *TrackingSimulator) SimulateDelivery(ctx context.Context, shipmentID string) error
```

**Event Types**:
- LABEL_CREATED
- PICKED_UP
- IN_TRANSIT
- OUT_FOR_DELIVERY
- DELIVERED
- EXCEPTION (delays, failed delivery attempts)
- RETURNED_TO_SENDER

### 6. Fault Injection Framework

**Location**: `/pkg/shipping/internal/fault/`

**FaultInjector** (`injector.go`):
```go
type FaultInjector struct {
    config     *FaultConfig
    scenarios  map[string]*FaultScenario
    random     *rand.Rand
}

func (f *FaultInjector) ShouldInjectFault(ctx context.Context, service string, operation string) (*Fault, bool)
func (f *FaultInjector) InjectLatency(ctx context.Context, baseDuration time.Duration) time.Duration
```

**Fault Types**:
- SERVICE_UNAVAILABLE (503 errors)
- TIMEOUT (request timeouts)
- RATE_LIMITED (429 errors)
- PARTIAL_FAILURE (some carriers work, others don't)
- DATA_CORRUPTION (invalid responses)
- SLOW_RESPONSE (artificial latency)

## Data Models

### Core Models

**Shipment** (`models/shipment.go`):
```go
type Shipment struct {
    ID             string                 `json:"id"`
    OrderID        string                 `json:"order_id"`
    CarrierID      string                 `json:"carrier_id"`
    ServiceType    string                 `json:"service_type"`
    TrackingNumber string                 `json:"tracking_number"`
    Status         ShipmentStatus         `json:"status"`
    LabelURL       string                 `json:"label_url"`
    FromAddress    *Address               `json:"from_address"`
    ToAddress      *Address               `json:"to_address"`
    Packages       []*Package             `json:"packages"`
    Rate           *Rate                  `json:"rate"`
    Metadata       map[string]interface{} `json:"metadata"`
    CreatedAt      time.Time              `json:"created_at"`
    ShippedAt      *time.Time             `json:"shipped_at"`
    DeliveredAt    *time.Time             `json:"delivered_at"`
}
```

**Rate** (`models/rate.go`):
```go
type Rate struct {
    CarrierID       string            `json:"carrier_id"`
    CarrierName     string            `json:"carrier_name"`
    ServiceType     string            `json:"service_type"`
    ServiceName     string            `json:"service_name"`
    Cost            float64           `json:"cost"`
    Currency        string            `json:"currency"`
    EstimatedDays   int               `json:"estimated_days"`
    CutoffTime      string            `json:"cutoff_time"`
    DeliveryDate    *time.Time        `json:"delivery_date"`
    Features        []string          `json:"features"`
    Metadata        map[string]string `json:"metadata"`
}
```

**Package** (`models/package.go`):
```go
type Package struct {
    ID          string      `json:"id"`
    ShipmentID  string      `json:"shipment_id"`
    Weight      float64     `json:"weight"`
    Dimensions  *Dimensions `json:"dimensions"`
    Contents    []*Item     `json:"contents"`
    Value       float64     `json:"value"`
    IsFragile   bool        `json:"is_fragile"`
    IsHazmat    bool        `json:"is_hazmat"`
    Temperature *TempRange  `json:"temperature"`
}
```

**TrackingEvent** (`models/tracking.go`):
```go
type TrackingEvent struct {
    ID          string                 `json:"id"`
    ShipmentID  string                 `json:"shipment_id"`
    Status      TrackingStatus         `json:"status"`
    Location    *Location              `json:"location"`
    Timestamp   time.Time              `json:"timestamp"`
    Description string                 `json:"description"`
    Metadata    map[string]interface{} `json:"metadata"`
}
```

## API Design

### Rate Calculation
```
POST /api/v1/shipping/rates
Content-Type: application/json

{
    "items": [
        {
            "id": "item-1",
            "weight": 2.5,
            "dimensions": {"length": 10, "width": 8, "height": 6},
            "value": 25.99,
            "fragile": false
        }
    ],
    "from_address": {
        "street": "123 Warehouse St",
        "city": "Portland",
        "state": "OR",
        "postal_code": "97201",
        "country": "US"
    },
    "to_address": {
        "street": "456 Customer Ave",
        "city": "Seattle",
        "state": "WA",
        "postal_code": "98101",
        "country": "US"
    },
    "services": ["standard", "express", "overnight"]
}

Response:
{
    "rates": [
        {
            "carrier_id": "ponyexpress",
            "carrier_name": "PonyExpress",
            "service_type": "standard_trot",
            "service_name": "Standard Trot",
            "cost": 8.99,
            "currency": "USD",
            "estimated_days": 3,
            "delivery_date": "2024-01-15T17:00:00Z",
            "features": ["tracking", "insurance"]
        },
        {
            "carrier_id": "catcarrier",
            "carrier_name": "CatCarrier",
            "service_type": "prowl_delivery",
            "service_name": "Prowl Delivery",
            "cost": 7.50,
            "currency": "USD",
            "estimated_days": 4,
            "delivery_date": "2024-01-16T17:00:00Z",
            "features": ["tracking"]
        }
    ],
    "recommendations": {
        "fastest": "ponyexpress:express_sprint",
        "cheapest": "turtletransit:economy_shell",
        "best_value": "catcarrier:prowl_delivery"
    }
}
```

### Shipment Creation
```
POST /api/v1/shipping/shipments
Content-Type: application/json

{
    "order_id": "order-12345",
    "carrier_id": "ponyexpress",
    "service_type": "standard_trot",
    "from_address": {...},
    "to_address": {...},
    "packages": [
        {
            "weight": 2.5,
            "dimensions": {"length": 10, "width": 8, "height": 6},
            "contents": [
                {
                    "name": "Premium Rope Toy",
                    "sku": "ROPE-TOY-LG",
                    "quantity": 1,
                    "value": 15.99
                }
            ],
            "fragile": false
        }
    ],
    "options": {
        "signature_required": true,
        "insurance": true,
        "delivery_confirmation": true
    }
}

Response:
{
    "shipment": {
        "id": "ship-abc123",
        "tracking_number": "PE123456789US",
        "label_url": "/api/v1/shipping/labels/ship-abc123",
        "status": "label_created",
        "estimated_delivery": "2024-01-15T17:00:00Z",
        "cost": 8.99
    }
}
```

### Tracking Information
```
GET /api/v1/shipping/track/PE123456789US

Response:
{
    "shipment": {
        "id": "ship-abc123",
        "tracking_number": "PE123456789US",
        "status": "in_transit",
        "estimated_delivery": "2024-01-15T17:00:00Z"
    },
    "events": [
        {
            "status": "label_created",
            "timestamp": "2024-01-12T10:00:00Z",
            "location": "Portland, OR",
            "description": "Shipping label created"
        },
        {
            "status": "picked_up",
            "timestamp": "2024-01-12T15:30:00Z",
            "location": "Portland, OR",
            "description": "Package picked up by PonyExpress"
        },
        {
            "status": "in_transit",
            "timestamp": "2024-01-13T08:00:00Z",
            "location": "Portland Distribution Center",
            "description": "Package in transit to destination"
        }
    ],
    "delivery_estimate": {
        "date": "2024-01-15",
        "time_window": "09:00-17:00",
        "confidence": "high"
    }
}
```

## OpenTelemetry Instrumentation Strategy

### Automatic Instrumentation
- HTTP middleware for all API endpoints
- Database operation spans (mock DB operations)
- External carrier API calls
- Cache operations

### Custom Metrics
```go
// Business Metrics
var (
    ShippingRateCalculations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "shipping_rate_calculations_total",
            Help: "Total number of rate calculations performed",
        },
        []string{"carrier", "service_type", "status"},
    )

    ShippingCosts = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "shipping_cost_dollars",
            Help: "Shipping costs in dollars",
            Buckets: []float64{5, 10, 20, 50, 100, 200},
        },
        []string{"carrier", "service_type", "zone"},
    )

    CarrierResponseTime = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "carrier_response_time_seconds",
            Help: "Carrier API response times",
            Buckets: prometheus.DefBuckets,
        },
        []string{"carrier", "operation", "status"},
    )

    FaultInjectionEvents = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "fault_injection_triggered_total",
            Help: "Total fault injection events triggered",
        },
        []string{"service", "fault_type"},
    )
)
```

### Trace Spans
```go
// Rate calculation span
ctx, span := tracer.Start(ctx, "shipping.calculate_rates")
span.SetAttributes(
    attribute.String("shipping.carrier", carrierID),
    attribute.String("shipping.service", serviceType),
    attribute.String("shipping.origin_zip", fromZip),
    attribute.String("shipping.dest_zip", toZip),
    attribute.Float64("shipping.weight", weight),
)
defer span.End()

// Carrier API call span
ctx, span := tracer.Start(ctx, "carrier.get_rates")
span.SetAttributes(
    attribute.String("carrier.name", carrier.Name()),
    attribute.String("carrier.operation", "get_rates"),
)
defer span.End()
```

## Configuration Files

### Carrier Configuration (`configs/carriers.yaml`)
```yaml
carriers:
  ponyexpress:
    name: "PonyExpress"
    enabled: true
    base_url: "https://api.ponyexpress.mock"
    characteristics:
      reliability: 0.95
      speed_multiplier: 1.2
      cost_multiplier: 1.1
    services:
      - id: "standard_trot"
        name: "Standard Trot"
        days: 3
        cutoff_time: "14:00"
        base_rate: 8.99
      - id: "priority_gallop"
        name: "Priority Gallop"
        days: 2
        cutoff_time: "14:00"
        base_rate: 12.99
      - id: "express_sprint"
        name: "Express Sprint"
        days: 1
        cutoff_time: "12:00"
        base_rate: 24.99
    zones:
      zone1: { multiplier: 1.0 }
      zone2: { multiplier: 1.2 }
      zone3: { multiplier: 1.5 }

  catcarrier:
    name: "CatCarrier"
    enabled: true
    characteristics:
      reliability: 0.85
      mood_factor: true  # Random delays/pricing
      nap_probability: 0.2
    services:
      - id: "prowl_delivery"
        name: "Prowl Delivery"
        days: 4
        base_rate: 7.50
      - id: "pounce_express"
        name: "Pounce Express"
        days: 2
        base_rate: 11.99
      - id: "midnight_dash"
        name: "Midnight Dash"
        days: 1
        base_rate: 22.99

  avianair:
    name: "AvianAir"
    enabled: true
    characteristics:
      reliability: 0.90
      eco_friendly: true
      weather_dependent: true
    services:
      - id: "ground_waddle"
        name: "Ground Waddle"
        days: 5
        base_rate: 6.99
      - id: "sky_glide"
        name: "Sky Glide"
        days: 3
        base_rate: 9.99
      - id: "eagle_express"
        name: "Eagle Express"
        days: 1
        base_rate: 19.99

  turtletransit:
    name: "TurtleTransit"
    enabled: true
    characteristics:
      reliability: 0.99
      speed_multiplier: 0.5
      cost_multiplier: 0.7
    services:
      - id: "economy_shell"
        name: "Economy Shell"
        days: 8
        base_rate: 4.99
```

### Fault Injection Configuration (`configs/fault-injection.yaml`)
```yaml
fault_injection:
  enabled: true
  global_rate: 0.05  # 5% base fault rate

  carriers:
    ponyexpress:
      failure_rate: 0.02  # 2% failure rate
      latency_ms: 100
      faults:
        - type: "timeout"
          probability: 0.01
          duration_ms: 5000
        - type: "service_unavailable"
          probability: 0.01

    catcarrier:
      failure_rate: 0.15  # 15% - cats are unpredictable
      latency_ms: 200
      faults:
        - type: "mood_swing"
          probability: 0.10
          effects: ["pricing_change", "service_refusal"]
        - type: "nap_delay"
          probability: 0.05
          duration_ms: 3000

    avianair:
      failure_rate: 0.08
      latency_ms: 150
      faults:
        - type: "weather_delay"
          probability: 0.05
          seasonal: true
        - type: "migration_season"
          probability: 0.03
          duration_days: 30

    turtletransit:
      failure_rate: 0.01  # Very reliable
      latency_ms: 500     # Always slow
      faults:
        - type: "shell_maintenance"
          probability: 0.005
          duration_ms: 10000

  scenarios:
    - name: "peak_season"
      active: false
      multiplier: 2.0
      duration_days: 14
    - name: "network_issues"
      active: false
      global_latency_ms: 2000
```

## Testing Strategy

### Unit Testing Requirements

**Rate Engine Tests** (`test/unit/services/rates_test.go`):
```go
func TestRateEngine_CalculateRates(t *testing.T) {
    tests := []struct {
        name     string
        request  *RateRequest
        expected []*Rate
        wantErr  bool
    }{
        {
            name: "successful_rate_calculation",
            request: &RateRequest{
                Items: []*Item{{Weight: 2.0}},
                From:  &Address{PostalCode: "97201"},
                To:    &Address{PostalCode: "98101"},
            },
            expected: []*Rate{
                {CarrierID: "ponyexpress", Cost: 8.99},
                {CarrierID: "catcarrier", Cost: 7.50},
            },
            wantErr: false,
        },
        // ... more test cases
    }
}
```

**Carrier Mock Tests** (`test/unit/carriers/ponyexpress_test.go`):
```go
func TestPonyExpressClient_GetRates(t *testing.T) {
    client := &PonyExpressClient{
        config: &CarrierConfig{Reliability: 0.95},
        fault:  &fault.NullInjector{},
    }

    rates, err := client.GetRates(context.Background(), &RateRequest{...})

    assert.NoError(t, err)
    assert.Len(t, rates, 3) // standard, priority, express
    assert.Equal(t, "standard_trot", rates[0].ServiceType)
}
```

**Fault Injection Tests** (`test/unit/fault/injector_test.go`):
```go
func TestFaultInjector_ShouldInjectFault(t *testing.T) {
    injector := &FaultInjector{
        config: &FaultConfig{
            Carriers: map[string]*CarrierFaultConfig{
                "catcarrier": {FailureRate: 0.5},
            },
        },
    }

    faultCount := 0
    for i := 0; i < 1000; i++ {
        if fault, should := injector.ShouldInjectFault(context.Background(), "catcarrier", "get_rates"); should {
            faultCount++
            assert.NotNil(t, fault)
        }
    }

    // Should be approximately 50% ± tolerance
    assert.InDelta(t, 500, faultCount, 50)
}
```

### Integration Testing Requirements

**End-to-End Rate Shopping** (`test/integration/rates_test.go`):
```go
func TestShippingService_RateShoppingFlow(t *testing.T) {
    // Test complete rate shopping flow
    // 1. Calculate rates from all carriers
    // 2. Verify rate ordering
    // 3. Test fault injection scenarios
    // 4. Verify telemetry data
}
```

**Tracking Simulation** (`test/integration/tracking_test.go`):
```go
func TestTrackingSimulator_FullDeliveryFlow(t *testing.T) {
    // Test complete tracking simulation
    // 1. Create shipment
    // 2. Simulate tracking events
    // 3. Verify webhooks fired
    // 4. Test delivery completion
}
```

### Performance Testing Scenarios

1. **Rate Calculation Load Test**: 1000 concurrent rate requests
2. **Carrier Failure Resilience**: Test with 50% carrier failure rate
3. **Tracking Event Volume**: Simulate 10,000 tracking events/minute
4. **Memory Usage**: Verify no memory leaks during extended operation

## Implementation Order

1. **Phase 1 - Foundation** (Week 1)
   - Basic project structure and configuration
   - In-memory mock database implementation
   - Core data models and interfaces
   - OpenTelemetry setup

2. **Phase 2 - Carrier Layer** (Week 1-2)
   - Carrier interface definition
   - Mock carrier implementations with unique characteristics
   - Fault injection framework
   - Basic carrier manager

3. **Phase 3 - Core Services** (Week 2-3)
   - Rate calculation engine
   - Package optimization algorithms
   - Address validation service
   - Basic API handlers

4. **Phase 4 - Advanced Features** (Week 3-4)
   - Label generation system
   - Tracking simulation engine
   - Notification system
   - Full API implementation

5. **Phase 5 - Testing & Polish** (Week 4)
   - Comprehensive unit tests
   - Integration tests
   - Performance testing
   - Documentation completion

## Dependencies

### Required External Libraries
```go
// HTTP and routing
"github.com/gorilla/mux"
"github.com/gorilla/handlers"

// Configuration
"gopkg.in/yaml.v3"
"github.com/spf13/viper"

// OpenTelemetry
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/trace"
"go.opentelemetry.io/otel/metric"
"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Monitoring
"github.com/prometheus/client_golang"

// Testing
"github.com/stretchr/testify"
"github.com/golang/mock"

// Utilities
"github.com/google/uuid"
"github.com/skip2/go-qrcode"
```

### Internal Dependencies
- Common telemetry package
- Shared configuration utilities
- Mock database implementation
- Fault injection framework

This comprehensive design provides a solid foundation for implementing a realistic, observable, and testable shipping service that meets all the POC requirements while maintaining simplicity and focusing on the core functionality needed for the Griffin Commerce demo.