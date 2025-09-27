# Common Service Requirements

## Overview
The Common Service provides shared utilities, middleware, and infrastructure components used across all services in the Griffin Commerce dog product store monorepo.

## Package
`package common`

## Core Requirements

### Configuration Management
- Centralized configuration loading from environment variables
- Configuration validation and type safety
- Service-specific configuration namespaces
- Secret management integration (AWS Secrets Manager)
- Feature flags and toggles
- Runtime configuration updates without restart

### Logging & Observability
- Structured logging with JSON output
- Correlation ID generation and propagation
- Request/response logging middleware
- Error tracking and aggregation
- Performance metrics collection
- Distributed tracing support (OpenTelemetry)
- Custom business metrics
- Log levels: DEBUG, INFO, WARN, ERROR, FATAL

### Mock Database Layer (POC Mode)
- In-memory mock database implementation
- Thread-safe operations with sync.RWMutex
- Generic CRUD interface for all entities
- Optional JSON persistence for data recovery
- Bulk operations support
- Query filtering and sorting
- Transaction simulation
- Configurable latency injection

### Mock Cache Layer (POC Mode)
- In-memory cache implementation
- TTL support with automatic expiration
- LRU eviction policy
- Cache statistics tracking
- Key pattern matching for bulk operations
- Thread-safe concurrent access
- Cache warming from YAML files
- Memory usage limits

### HTTP Middleware
- Authentication/authorization middleware
- CORS configuration
- Request ID generation
- Request/response compression
- Rate limiting per endpoint
- Request validation
- Error handling and formatting
- Timeout handling

### Error Handling
- Custom error types with error codes
- Error wrapping with context
- HTTP error responses standardization
- Error recovery strategies
- Panic recovery middleware
- Error notification thresholds
- User-friendly error messages

### Validation Utilities
- Input validation helpers
- Email/phone validation
- Data sanitization
- Schema validation (JSON Schema)
- Business rule validation framework
- Validation error formatting

### Security Utilities
- JWT token generation/validation
- Password hashing (bcrypt)
- API key management
- HMAC signature verification
- Input sanitization
- SQL injection prevention
- XSS protection helpers

### Message Queue Abstractions
- Publisher/subscriber interfaces
- Message serialization/deserialization
- Dead letter queue handling
- Message retry logic
- Event sourcing utilities
- CQRS command/query separation

### Shared Data Models
- Customer entity (ID, Email, Name, Addresses)
- Product entity (ID, SKU, Name, Price, Category)
- Order entity (ID, CustomerID, Status, Total)
- Address entity (Street, City, State, PostalCode, Country)
- Money type with currency handling
- Timestamp utilities with timezone support

### Testing Utilities
- Test database setup/teardown
- Mock data generators
- HTTP test client
- Integration test helpers
- Benchmark utilities
- Test coverage reporting

### API Response Standards
- Consistent response envelope structure
- Pagination utilities
- Sorting/filtering helpers
- API versioning support
- Response compression
- ETag generation

### Health & Monitoring
- Health check endpoints
- Readiness/liveness probes
- Dependency health aggregation
- Graceful shutdown handling
- Resource usage monitoring
- Service discovery registration

### Development Tools
- Code generation templates
- API documentation generation
- Development server with hot reload
- Database seeding utilities
- Performance profiling helpers
- Debug mode toggles

## Implementation Guidelines

### POC Mode Features
- All external dependencies replaced with mocks
- Configurable fault injection for testing
- Full OpenTelemetry instrumentation
- YAML-driven configuration and data loading
- In-memory storage with optional persistence

### No Backward Compatibility
- Breaking changes are allowed at any time
- No version migration paths needed
- Clean up deprecated code immediately
- Refactor freely without legacy constraints

### Monorepo Structure
```
/common
  /config     - Configuration management (YAML loading)
  /mockdb     - Mock database implementation
  /mockcache  - Mock cache implementation
  /fault      - Fault injection framework
  /telemetry  - OpenTelemetry setup and helpers
  /http       - HTTP middleware and utilities
  /errors     - Error handling
  /logging    - Structured logging with OTEL integration
  /security   - Security utilities
  /models     - Shared data models
  /testing    - Test utilities and helpers
  /monitoring - Health and monitoring endpoints
```

### Service Integration
- All services must import common package
- Use dependency injection for common components
- Maintain service isolation despite shared code
- Common package has no service-specific dependencies

## Performance Requirements
- Middleware overhead: < 1ms per request
- Cache operations: < 10ms
- Database connection pool: 100 connections max
- Logging should not block request processing
- Configuration loading: < 100ms on startup

## Notes
- No backward compatibility required (greenfield project)
- Part of isolated monorepo architecture
- Keep common package lightweight and focused
- Avoid business logic in common package
- Use Go interfaces for extensibility
- Minimize external dependencies