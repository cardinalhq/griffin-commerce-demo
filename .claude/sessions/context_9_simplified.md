# Simplified POC Requirements - Griffin Commerce Demo

## Overview
A simplified e-commerce demo focusing on demonstrating complex service interactions without unnecessary features. The goal is to show microservices communication patterns, fault tolerance, and observability.

## Core Architecture Principles

### Keep
- Individual microservices (7 services)
- Service-to-service communication
- Basic fault injection (simple % failure rates)
- OpenTelemetry tracing
- YAML configuration
- In-memory data storage

### Remove
- A/B testing
- Event bus/async messaging
- All caching layers
- Guest cart merging
- Complex mocking requirements
- Advanced integration patterns

## Service Architecture

**Maintaining Original 7 Services:**
1. Common Service - Shared utilities
2. Product Catalog Service - Product management
3. Shopping Cart Service - Cart operations
4. Payment Service - Payment processing
5. Shipping Service - Shipping operations
6. Image Service - Image serving
7. Recommendations Service - Product recommendations

## Service Specifications

### 1. Common Service
**Package:** `common`

**Simplified Scope:**
- Configuration loading from YAML
- Basic in-memory database (simple map with mutex)
- HTTP middleware (logging, tracing, correlation ID)
- Simple error types
- Basic OpenTelemetry setup (HTTP tracing only)

**Remove:**
- Cache layer
- Circuit breakers
- Complex fault injection patterns
- Hot reload configuration

### 2. Product Catalog Service
**Package:** `catalog`
**Port:** 8080

**Simplified Product Model:**
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
```

**API Endpoints:**
- GET `/api/products` - List all products
- GET `/api/products/{id}` - Get single product
- POST `/api/products/{id}/reserve` - Reserve stock for order
- POST `/api/products/{id}/release` - Release reserved stock

**Remove:**
- Product variants
- Complex attributes
- Search/filtering (just return all)
- Multiple images
- Dimensions/weight

### 3. Shopping Cart Service
**Package:** `cart`
**Port:** 8082

**Simplified Cart Model:**
```go
type Cart struct {
    ID         string
    CustomerID string  // Always required, no guest carts
    Items      []CartItem
    Total      float64
}

type CartItem struct {
    ProductID string
    Quantity  int
    Price     float64
}
```

**API Endpoints:**
- POST `/api/cart/create` - Create cart for customer
- POST `/api/cart/{id}/add` - Add item
- DELETE `/api/cart/{id}/item/{productId}` - Remove item
- GET `/api/cart/{id}` - Get cart
- POST `/api/cart/{id}/checkout` - Begin checkout

**Remove:**
- Guest carts and merging
- Promotions/discounts
- Cart persistence beyond session
- Complex calculations (just sum prices)

### 4. Payment Service
**Package:** `payment`
**Port:** 8081

**Simplified to 3 Payment Processors:**
```yaml
processors:
  puppypay:
    failure_rate: 0.05  # 5% failure
    latency_ms: 100
  kittycard:
    failure_rate: 0.20  # 20% failure for testing
    latency_ms: 150
  doggiecoin:
    failure_rate: 0.10  # 10% failure
    latency_ms: 50
```

**Simplified Transaction Model:**
```go
type Transaction struct {
    ID         string
    OrderID    string
    Amount     float64
    Status     string  // "success", "failed"
    Processor  string
}
```

**API Endpoints:**
- POST `/api/payments/charge` - Process payment
- GET `/api/payments/{id}` - Get transaction status

**Remove:**
- Authorize/capture flow (just direct charge)
- Refunds
- Payment method tokenization
- Webhooks

### 5. Shipping Service
**Package:** `shipping`
**Port:** 8084

**Simplified to 3 Carriers:**
```yaml
carriers:
  ponyexpress:
    rate: 9.99
    failure_rate: 0.05  # 5% failure
    name: "Pony Express Ground"
  avianair:
    rate: 19.99
    failure_rate: 0.10  # 10% failure
    name: "Avian Air Express"
  catcarrier:
    rate: 14.99
    failure_rate: 0.25  # 25% failure (cats are unpredictable)
    name: "Cat Carrier Delivery"
```

**Simplified Shipment Model:**
```go
type Shipment struct {
    ID        string
    OrderID   string
    Carrier   string
    Status    string  // "submitted", "failed"
    Cost      float64
}
```

**API Endpoints:**
- POST `/api/shipping/calculate` - Get rates for all carriers
- POST `/api/shipping/submit` - Submit shipment (may fail based on carrier)

**Remove:**
- Label generation
- Package optimization
- Tracking updates
- Address validation
- Multi-package shipments

### 6. Recommendations Service (Simplified)
**Package:** `recommendations`
**Port:** 8085

**Simplified Scope:**
- Return random "frequently bought together" products
- No user profiling or ML
- Simple static rules

**API Endpoints:**
- GET `/api/recommendations/product/{id}` - Get 3 random related products
- GET `/api/recommendations/cart/{id}` - Get 3 random products not in cart

**Remove:**
- User profiles
- Purchase history tracking
- ML algorithms
- Personalization
- A/B testing

### 7. Image Service (Simplified to Static)
**Package:** `images`
**Port:** 8083

**Simplified Scope:**
- Serve static product images from `/static` directory
- No processing, resizing, or CDN simulation
- Just return image URLs

**Remove:**
- Image processing
- CDN simulation
- Multiple formats
- On-demand resizing

## Fault Injection (Simplified)

**Basic Random Failures Only:**
```yaml
fault_injection:
  enabled: true
  services:
    payment:
      failure_rate: 0.1  # 10% of all payment requests fail
    shipping:
      failure_rate: 0.15 # 15% of shipping submissions fail
```

**Remove:**
- Pattern-based failures
- Specific failure conditions
- Latency injection
- Complex retry logic

## OpenTelemetry (Simplified)

**Basic Tracing Only:**
- HTTP middleware auto-instrumentation
- Span creation for service calls
- Context propagation
- Console exporter (no OTLP)

**Remove:**
- Custom business metrics
- Complex span attributes
- Performance metrics
- Log correlation

## Data Persistence (Simplified)

**Basic In-Memory Maps:**
```go
type Database struct {
    mu       sync.RWMutex
    products map[string]*Product
    carts    map[string]*Cart
    orders   map[string]*Order
}
```

**Remove:**
- JSON file persistence
- Query capabilities
- Transaction support
- Indexes

## Configuration (Simplified)

**Single config.yaml:**
```yaml
services:
  catalog:
    port: 8080
  cart:
    port: 8082
  payment:
    port: 8081
    processors:
      - puppypay
      - kittycard
      - doggiecoin
  shipping:
    port: 8084
    carriers:
      - ponyexpress
      - avianair
      - catcarrier
  image:
    port: 8083
  recommendations:
    port: 8085

fault_injection:
  payment_failure_rate: 0.1
  shipping_failure_rate: 0.15

telemetry:
  enabled: true
  service_name: "griffin-commerce"
```

## Testing Requirements (Simplified)

**Focus Areas:**
1. Service communication works
2. Failures are handled gracefully
3. Tracing shows request flow
4. Basic retry logic works

**Remove:**
- Comprehensive unit tests
- Performance testing
- Load testing
- Integration test suites

## Success Criteria

The POC demonstrates:
1. **Service Communication:** Services successfully call each other
2. **Fault Tolerance:** System handles failures without crashing
3. **Observability:** Can trace requests across services
4. **Configuration:** Services configured via YAML
5. **Simplicity:** Can be understood and modified easily

## Implementation Priority

1. **Week 1:**
   - Common Service (basic utilities)
   - Product Catalog Service (load from YAML)
   - Cart Service (basic operations)
   - Image Service (static files)

2. **Week 2:**
   - Payment Service (with failure simulation)
   - Shipping Service (with failure simulation)
   - Recommendations Service (random products)
   - Basic OpenTelemetry integration

## What This Achieves

- **Demonstrates microservices patterns** without unnecessary complexity
- **Shows fault tolerance** through simple retry and error handling
- **Provides observability** through basic tracing
- **Maintains separate services** to show communication patterns
- **Reduces implementation time** from 6 weeks to 2 weeks
- **Focuses on core POC goals** rather than feature completeness