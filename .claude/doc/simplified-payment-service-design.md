# Simplified Payment Service System Design

## Overview

The Simplified Payment Service is a minimal implementation for the Griffin Commerce demo POC. This design eliminates complex features and focuses on basic payment processing with random failure simulation. The service can be implemented in 1 day and demonstrates microservice communication patterns without unnecessary complexity.

## System Architecture

### High-Level Design

```text
┌─────────────────────────────────────────────┐
│            Payment Service                  │
│                 (Port 8081)                 │
├─────────────────────────────────────────────┤
│  ┌─────────────┐    ┌─────────────────┐     │
│  │ HTTP API    │    │ Mock Processor  │     │
│  │ Handlers    │    │ Manager         │     │
│  └─────────────┘    └─────────────────┘     │
├─────────────────────────────────────────────┤
│  ┌─────────────┐    ┌─────────────────┐     │
│  │ Transaction │    │ Random Failure  │     │
│  │ Storage     │    │ Simulator       │     │
│  │ (Memory)    │    │                 │     │
│  └─────────────┘    └─────────────────┘     │
└─────────────────────────────────────────────┘
```

## File Structure (FLAT DESIGN)

```text
payment/
├── main.go        # Server setup and initialization
├── handlers.go    # HTTP request handlers
├── processor.go   # Mock processor logic and failure simulation
├── config.yaml    # Processor configuration
└── README.md      # Basic usage instructions
```

**No subdirectories, no complex layering - keep it flat and simple.**

## Data Models

### Simplified Transaction Model

```go
type Transaction struct {
    ID        string    `json:"id"`
    OrderID   string    `json:"order_id"`
    Amount    float64   `json:"amount"`
    Status    string    `json:"status"`    // "success" or "failed"
    Processor string    `json:"processor"` // which processor was used
    CreatedAt time.Time `json:"created_at"`
}
```

**That's it - no complex statuses, no currency fields, no metadata, no refunds.**

### Processor Configuration

```go
type ProcessorConfig struct {
    Name        string  `yaml:"name"`
    FailureRate float64 `yaml:"failure_rate"` // 0.0 to 1.0
    LatencyMs   int     `yaml:"latency_ms"`   // milliseconds
}
```

## API Design

### Endpoints (Minimal)

#### 1. Process Payment (Charge)
```http
POST /api/payments/charge
Content-Type: application/json

{
  "order_id": "ORDER-123",
  "amount": 99.99,
  "processor": "puppypay"  // optional, random if not specified
}
```

**Success Response:**
```json
{
  "transaction_id": "txn_abc123",
  "order_id": "ORDER-123",
  "amount": 99.99,
  "status": "success",
  "processor": "puppypay",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Failure Response:**
```json
{
  "transaction_id": "txn_abc123",
  "order_id": "ORDER-123",
  "amount": 99.99,
  "status": "failed",
  "processor": "kittycard",
  "created_at": "2024-01-15T10:30:00Z"
}
```

#### 2. Get Transaction Status
```http
GET /api/payments/{transaction_id}
```

**Response:**
```json
{
  "transaction_id": "txn_abc123",
  "order_id": "ORDER-123",
  "amount": 99.99,
  "status": "success",
  "processor": "puppypay",
  "created_at": "2024-01-15T10:30:00Z"
}
```

**Error Response:**
```json
{
  "error": "transaction not found"
}
```

## Mock Processor Implementation

### Processor Configuration (config.yaml)

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

### Processor Logic

```go
type Processor struct {
    Name        string
    FailureRate float64
    LatencyMs   int
}

func (p *Processor) ProcessPayment(amount float64) (bool, error) {
    // 1. Sleep for latency simulation
    time.Sleep(time.Duration(p.LatencyMs) * time.Millisecond)

    // 2. Random failure simulation
    if rand.Float64() < p.FailureRate {
        return false, nil // Payment failed
    }

    return true, nil // Payment successful
}
```

### Processor Selection Logic

```go
func SelectProcessor(requestedProcessor string, processors []Processor) Processor {
    // If specific processor requested, use it
    if requestedProcessor != "" {
        for _, p := range processors {
            if p.Name == requestedProcessor {
                return p
            }
        }
    }

    // Otherwise, pick random processor
    return processors[rand.Intn(len(processors))]
}
```

## Failure Simulation Approach

### Simple Random Failures

The failure simulation is intentionally simple:

1. **Random Number Generation**: Each payment request generates a random number (0.0 to 1.0)
2. **Threshold Comparison**: If random number < failure_rate, the payment fails
3. **No Complex Patterns**: No time-based failures, no specific card numbers, no fraud simulation
4. **No Retry Logic**: One attempt only - if it fails, it fails

### Failure Rate Examples

- **PuppyPay (5% failure)**: 95% of payments succeed, 5% fail randomly
- **KittyCard (20% failure)**: 80% of payments succeed, 20% fail randomly
- **DoggieCoin (10% failure)**: 90% of payments succeed, 10% fail randomly

### Latency Simulation

Each processor has a fixed latency:
- **PuppyPay**: 100ms delay
- **KittyCard**: 150ms delay
- **DoggieCoin**: 50ms delay

## In-Memory Storage

### Simple Map-Based Storage

```go
type PaymentStorage struct {
    transactions map[string]*Transaction
    mutex        sync.RWMutex
}

func (s *PaymentStorage) Store(txn *Transaction) {
    s.mutex.Lock()
    defer s.mutex.Unlock()
    s.transactions[txn.ID] = txn
}

func (s *PaymentStorage) Get(id string) (*Transaction, bool) {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    txn, exists := s.transactions[id]
    return txn, exists
}
```

**No persistence, no complex queries, no relationships - just a thread-safe map.**

## Implementation Details

### main.go Structure

```go
package main

import (
    "log"
    "net/http"
    "github.com/gorilla/mux"
)

func main() {
    // 1. Load configuration from config.yaml
    config := loadConfig()

    // 2. Initialize processors
    processors := initializeProcessors(config)

    // 3. Initialize storage
    storage := &PaymentStorage{
        transactions: make(map[string]*Transaction),
    }

    // 4. Setup HTTP routes
    router := mux.NewRouter()
    router.HandleFunc("/api/payments/charge", chargeHandler(processors, storage)).Methods("POST")
    router.HandleFunc("/api/payments/{id}", getTransactionHandler(storage)).Methods("GET")

    // 5. Start server
    log.Println("Payment service starting on port 8081")
    log.Fatal(http.ListenAndServe(":8081", router))
}
```

### handlers.go Implementation

```go
func chargeHandler(processors []Processor, storage *PaymentStorage) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Parse request
        var req ChargeRequest
        json.NewDecoder(r.Body).Decode(&req)

        // 2. Generate transaction ID
        txnID := generateID()

        // 3. Select processor
        processor := SelectProcessor(req.Processor, processors)

        // 4. Process payment
        success, _ := processor.ProcessPayment(req.Amount)

        // 5. Create transaction record
        txn := &Transaction{
            ID:        txnID,
            OrderID:   req.OrderID,
            Amount:    req.Amount,
            Status:    getStatus(success),
            Processor: processor.Name,
            CreatedAt: time.Now(),
        }

        // 6. Store transaction
        storage.Store(txn)

        // 7. Return response
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(txn)
    }
}
```

## Integration with Common Service

### Configuration Loading

The payment service will use the Common Service for:

1. **YAML Configuration Loading**: Load processor config from config.yaml
2. **Basic HTTP Middleware**: Request logging and correlation IDs
3. **Error Response Formatting**: Consistent error response structure

### Service Registration

```go
// Register with common service discovery (if implemented)
common.RegisterService("payment", "8081", "/health")
```

### Shared Utilities

```go
import "github.com/griffin-commerce/common"

// Use common utilities for:
// - ID generation
// - Request logging
// - Error handling
// - Configuration parsing
```

## Testing Strategy

### Unit Tests (Keep Minimal)

```go
func TestProcessorFailureSimulation(t *testing.T) {
    processor := Processor{
        Name: "test",
        FailureRate: 0.5, // 50% failure rate
        LatencyMs: 10,
    }

    successCount := 0
    failureCount := 0

    // Run 1000 simulations
    for i := 0; i < 1000; i++ {
        success, _ := processor.ProcessPayment(100.0)
        if success {
            successCount++
        } else {
            failureCount++
        }
    }

    // Should be approximately 50/50 split
    assert.InDelta(t, 500, successCount, 100)
    assert.InDelta(t, 500, failureCount, 100)
}
```

### Integration Tests

```go
func TestChargeEndpoint(t *testing.T) {
    // Test successful payment
    req := ChargeRequest{
        OrderID: "TEST-123",
        Amount: 99.99,
        Processor: "puppypay", // Force specific processor
    }

    resp := callChargeAPI(req)
    assert.Equal(t, "success", resp.Status) // Should succeed with puppypay
    assert.Equal(t, "puppypay", resp.Processor)
}
```

## Health Check

### Simple Health Endpoint

```go
func healthHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "healthy",
            "service": "payment",
            "version": "1.0.0",
        })
    }
}
```

## Error Handling (Simplified)

### Basic Error Responses

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code,omitempty"`
    Details string `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, message string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(ErrorResponse{
        Error: message,
    })
}
```

## Implementation Timeline

### Day 1 (1 day implementation)

1. **Hour 1-2**: Set up project structure and basic HTTP server
2. **Hour 3-4**: Implement processor configuration and selection logic
3. **Hour 5-6**: Add failure simulation and latency logic
4. **Hour 7-8**: Create HTTP handlers for charge and get endpoints
5. **Hour 9**: Add basic tests and health check
6. **Hour 10**: Integration testing and bug fixes

### What This Achieves

1. **Demonstrates Payment Processing**: Shows how payments work in a microservice
2. **Shows Failure Handling**: Other services can handle payment failures
3. **Enables End-to-End Testing**: Cart service can call payment service
4. **Provides Realistic Latency**: Services experience real network delays
5. **Simple Configuration**: Easy to adjust failure rates for testing

### What We Deliberately Don't Include

- ❌ Authorization/capture flow
- ❌ Refunds or voids
- ❌ Payment method tokenization
- ❌ Webhooks or async notifications
- ❌ Complex retry logic
- ❌ Circuit breakers
- ❌ Detailed metrics beyond basic success/failure
- ❌ Fraud detection
- ❌ Multiple currencies
- ❌ Partial payments
- ❌ Complex error codes

## Dependencies

### Minimal External Dependencies

```go
// Essential only
"github.com/gorilla/mux"     // HTTP routing
"gopkg.in/yaml.v3"           // YAML config loading
"github.com/google/uuid"     // ID generation

// From common service
"github.com/griffin-commerce/common"
```

### No Complex Dependencies

- No ORM or database drivers
- No complex HTTP clients
- No async messaging
- No external payment SDKs
- No cryptographic libraries
- No monitoring frameworks

## Success Criteria

The simplified payment service succeeds if:

1. ✅ **It processes payments**: POST to /charge returns success or failure
2. ✅ **It simulates failures**: Failure rates approximately match configuration
3. ✅ **It stores transactions**: GET endpoint returns stored transaction data
4. ✅ **It has realistic latency**: Processors add configured delays
5. ✅ **It's simple to understand**: Any developer can read and modify the code
6. ✅ **It integrates easily**: Other services can call it via HTTP
7. ✅ **It can be deployed**: Runs as a standalone service on port 8081

This design prioritizes simplicity and implementability over feature completeness, making it perfect for a 2-week POC that demonstrates microservice patterns without unnecessary complexity.