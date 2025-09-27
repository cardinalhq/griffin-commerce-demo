# Image Service Design - Simplified Implementation

## Overview

The Image Service is an extremely simplified static file server for the Griffin Commerce demo. Its sole purpose is to serve product images from a static directory and provide a simple API to map product IDs to image URLs. This service prioritizes simplicity over features and could essentially be replaced with nginx.

## System Architecture

### High-Level Design
```
┌─────────────────┐
│   Image Service │
│   (Port 8083)   │
├─────────────────┤
│  HTTP Router    │
├─────────────────┤
│ Static Handler  │
│ Product Mapper  │
├─────────────────┤
│ Static Files    │
│ (/static dir)   │
└─────────────────┘
```

### Component Responsibilities

1. **HTTP Router**: Routes requests to appropriate handlers
2. **Static File Handler**: Serves files directly from `/static` directory using Go's built-in `http.FileServer`
3. **Product Mapper**: Maps product IDs to static image filenames
4. **Static Directory**: Contains all product image files

## File Structure

```
images/
├── main.go                 # Server setup and routing
├── handlers.go             # HTTP request handlers
├── product_mapper.go       # Product ID to image mapping
├── config.yaml            # Simple configuration
├── go.mod                  # Go module definition
└── static/                 # Static image files directory
    ├── rope-toy.jpg        # Product images
    ├── bacon-treats.jpg
    ├── squeaky-ball.jpg
    ├── training-leash.jpg
    └── dental-chews.jpg
```

## Data Models

### Product Image Mapping
```go
// Simple map of product ID to image filename
type ProductImageMap map[string]string

// Example data
var productImages = ProductImageMap{
    "PROD-001": "rope-toy.jpg",
    "PROD-002": "bacon-treats.jpg",
    "PROD-003": "squeaky-ball.jpg",
    "PROD-004": "training-leash.jpg",
    "PROD-005": "dental-chews.jpg",
}
```

### API Response Format
```go
type ImageResponse struct {
    ProductID string `json:"product_id"`
    ImageURL  string `json:"image_url"`
}
```

## API Specifications

### Endpoints

#### 1. Serve Static Files
- **Method**: GET
- **Path**: `/static/{filename}`
- **Purpose**: Serve static image files directly
- **Handler**: Go's `http.FileServer`
- **Example**: `GET /static/rope-toy.jpg`
- **Response**: Raw image file with appropriate Content-Type headers

#### 2. Get Product Image URL
- **Method**: GET
- **Path**: `/api/images/product/{id}`
- **Purpose**: Return image URL for a given product ID
- **Example Request**: `GET /api/images/product/PROD-001`
- **Example Response**:
```json
{
    "product_id": "PROD-001",
    "image_url": "/static/rope-toy.jpg"
}
```

#### 3. Health Check (Optional)
- **Method**: GET
- **Path**: `/health`
- **Response**: `{"status": "ok"}`

### Error Handling

#### Product Not Found
- **Status**: 404 Not Found
- **Response**:
```json
{
    "error": "product not found",
    "product_id": "PROD-999"
}
```

#### File Not Found
- **Status**: 404 Not Found
- **Response**: Standard HTTP 404 from file server

## Implementation Details

### Static File Serving
Use Go's built-in `http.FileServer` to serve files from the `/static` directory:

```go
// Serve static files from /static directory
fs := http.FileServer(http.Dir("./static/"))
http.Handle("/static/", http.StripPrefix("/static/", fs))
```

### Product ID Mapping
Simple in-memory map lookup:

```go
func getProductImage(productID string) (string, bool) {
    filename, exists := productImages[productID]
    return filename, exists
}
```

### HTTP Handler Structure
```go
func handleProductImage(w http.ResponseWriter, r *http.Request) {
    // Extract product ID from URL
    // Look up image filename
    // Return JSON response with image URL
}
```

### Configuration
Simple YAML configuration:
```yaml
server:
  port: 8083
  static_dir: "./static"

images:
  base_url: "/static"
```

## Deployment Configuration

### Port Assignment
- **Port**: 8083 (as specified in requirements)
- **Static Route**: `/static/*`
- **API Route**: `/api/images/*`

### Directory Setup
```bash
mkdir -p images/static
cd images
# Place image files in static/ directory
# All images should be web-optimized (JPEG/PNG)
```

## Testing Strategy

### Unit Tests Required

#### 1. Product Mapping Tests
```go
func TestProductImageMapping(t *testing.T) {
    // Test valid product ID returns correct filename
    // Test invalid product ID returns not found
    // Test all expected products have mappings
}
```

#### 2. HTTP Handler Tests
```go
func TestProductImageHandler(t *testing.T) {
    // Test GET /api/images/product/{id} returns correct JSON
    // Test invalid product ID returns 404
    // Test malformed requests return 400
}
```

#### 3. Static File Serving Tests
```go
func TestStaticFileServing(t *testing.T) {
    // Test GET /static/{filename} serves correct file
    // Test missing files return 404
    // Test correct Content-Type headers
}
```

#### 4. Integration Tests
```go
func TestEndToEnd(t *testing.T) {
    // Test complete flow: product API -> static file serving
    // Test server startup and configuration loading
}
```

### Test Data
```
test_static/
├── test-image-1.jpg
├── test-image-2.png
└── missing-file-test.jpg (intentionally missing)
```

## Dependencies

### External Libraries (Minimal)
```go
// go.mod
module github.com/cardinalhq/griffin-commerce-demo/images

go 1.21

require (
    gopkg.in/yaml.v3 v3.0.1  // For config loading
)
```

### Standard Library Usage
- `net/http` - HTTP server and file serving
- `encoding/json` - JSON response encoding
- `path/filepath` - File path manipulation
- `log` - Basic logging

## Implementation Order

### Phase 1: Basic Static File Server (30 minutes)
1. Create `main.go` with basic HTTP server
2. Set up static file serving with `http.FileServer`
3. Test serving files from `/static` directory

### Phase 2: Product Mapping API (30 minutes)
1. Create `product_mapper.go` with static mapping
2. Implement `handleProductImage` handler
3. Add JSON response formatting

### Phase 3: Configuration and Error Handling (30 minutes)
1. Add YAML configuration loading
2. Implement proper error responses
3. Add basic logging

### Phase 4: Testing (30 minutes)
1. Write unit tests for all components
2. Create test data and fixtures
3. Verify integration works end-to-end

## Monitoring and Observability

### Basic Logging
```go
// Log all requests with basic info
log.Printf("GET %s - %d", r.URL.Path, statusCode)
```

### Health Check
Simple health endpoint to verify service is running:
```go
func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

## Security Considerations

### Path Traversal Prevention
The `http.FileServer` automatically prevents path traversal attacks, but additional validation can be added:

```go
// Validate filename doesn't contain path traversal
if strings.Contains(filename, "..") {
    http.Error(w, "Invalid filename", http.StatusBadRequest)
    return
}
```

### File Type Restrictions
Only serve known image file types:
```go
validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
```

## Performance Characteristics

### Expected Load
- **Concurrent Requests**: Low (demo environment)
- **File Size Range**: 50KB - 500KB per image
- **Cache Headers**: Basic cache headers for static files

### Optimization Notes
- No optimization required for demo
- Files served directly from disk
- No image processing or resizing
- Could add basic cache headers if needed

## Future Considerations (NOT Implemented)

These features are explicitly excluded from this simplified design:

- ❌ Image processing/resizing
- ❌ CDN simulation
- ❌ Multiple image formats per product
- ❌ Image upload capabilities
- ❌ Caching layers
- ❌ Image optimization
- ❌ Watermarking
- ❌ Dynamic image generation

## Success Criteria

The Image Service is successful when:

1. **Static Files Served**: Images accessible via `/static/{filename}`
2. **API Functional**: Product image URLs returned via `/api/images/product/{id}`
3. **Error Handling**: Proper 404s for missing products/files
4. **Configuration**: Service configurable via YAML
5. **Testing**: All components have unit tests
6. **Integration**: Works with Product Catalog Service
7. **Simplicity**: Implementation under 200 lines of code

## Total Implementation Time
**Estimated**: 2 hours maximum
- 30 minutes: Basic static file serving
- 30 minutes: Product mapping API
- 30 minutes: Configuration and error handling
- 30 minutes: Testing and documentation

This design prioritizes extreme simplicity and rapid implementation while meeting all core requirements for the Griffin Commerce demo.