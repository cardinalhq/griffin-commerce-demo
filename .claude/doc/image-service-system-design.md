# Image Service System Design

## Overview

The Image Service is a critical component of the Griffin Commerce demo that handles all image-related operations including storage, processing, optimization, and CDN delivery simulation. This service provides high-performance image operations with full observability and mock external dependencies for POC demonstration.

## System Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   API Gateway   │───▶│   Image Service   │───▶│  Mock CDN/      │
│                 │    │                  │    │  File Storage   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │  Processing      │
                       │  Pipeline        │
                       └──────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │  In-Memory       │
                       │  Cache           │
                       └──────────────────┘
```

### Component Architecture

```
image/
├── api/                    # HTTP handlers and middleware
│   ├── handlers.go        # Image upload, retrieval, processing endpoints
│   ├── middleware.go      # Authentication, rate limiting, instrumentation
│   └── validation.go      # Request validation and sanitization
├── service/               # Business logic layer
│   ├── image_service.go   # Core image operations
│   ├── processor.go       # Image processing pipeline
│   ├── cdn.go            # Mock CDN implementation
│   └── batch.go          # Batch processing operations
├── storage/               # Storage abstraction layer
│   ├── file_storage.go    # Local filesystem storage
│   ├── cache.go          # In-memory cache implementation
│   └── metadata.go       # Image metadata management
├── models/                # Data models
│   ├── image.go          # Image entity models
│   ├── request.go        # API request/response models
│   └── config.go         # Configuration models
├── processing/            # Image processing utilities
│   ├── resize.go         # Resizing and cropping logic
│   ├── optimize.go       # Format conversion and optimization
│   ├── validator.go      # Image validation and security
│   └── effects.go        # Blur placeholders, filters
├── mockcdn/               # FurryFast CDN simulation
│   ├── cdn_server.go     # Mock CDN HTTP server
│   ├── latency.go        # Geographic latency simulation
│   └── failover.go       # CDN failover simulation
├── mockdb/                # Mock database implementation
│   ├── image_db.go       # Image metadata storage
│   └── db.go             # Generic mock DB interface
├── telemetry/             # OpenTelemetry instrumentation
│   ├── metrics.go        # Custom metrics collection
│   ├── tracing.go        # Distributed tracing
│   └── logging.go        # Structured logging
└── config/                # Configuration management
    ├── config.go         # Configuration loading
    └── validation.go     # Configuration validation
```

## Data Models

### Core Models

```go
// Image represents a stored image with metadata
type Image struct {
    ID          string    `json:"id"`
    ProductID   string    `json:"product_id"`
    Type        string    `json:"type"` // primary, gallery, variant
    OriginalURL string    `json:"original_url"`
    FileName    string    `json:"filename"`
    ContentType string    `json:"content_type"`
    FileSize    int64     `json:"file_size"`
    Width       int       `json:"width"`
    Height      int       `json:"height"`
    Format      string    `json:"format"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    Metadata    ImageMetadata `json:"metadata"`
    Variants    []ImageVariant `json:"variants"`
}

// ImageMetadata contains detailed image information
type ImageMetadata struct {
    ColorProfile string            `json:"color_profile"`
    EXIF         map[string]string `json:"exif"`
    DominantColor string           `json:"dominant_color"`
    BlurHash     string            `json:"blur_hash"`
    AltText      string            `json:"alt_text"`
}

// ImageVariant represents a processed version of an image
type ImageVariant struct {
    ID           string    `json:"id"`
    ParentID     string    `json:"parent_id"`
    URL          string    `json:"url"`
    Width        int       `json:"width"`
    Height       int       `json:"height"`
    Format       string    `json:"format"`
    Quality      int       `json:"quality"`
    FileSize     int64     `json:"file_size"`
    CacheKey     string    `json:"cache_key"`
    LastAccessed time.Time `json:"last_accessed"`
}

// ProcessingRequest defines image transformation parameters
type ProcessingRequest struct {
    Width   int    `json:"width"`
    Height  int    `json:"height"`
    Format  string `json:"format"`
    Quality int    `json:"quality"`
    Fit     string `json:"fit"` // cover, contain, fill, inside, outside
}
```

### Configuration Models

```go
// Config represents the image service configuration
type Config struct {
    Port         int                `yaml:"port"`
    StoragePath  string            `yaml:"storage_path"`
    CDNURL       string            `yaml:"cdn_url"`
    Cache        CacheConfig       `yaml:"cache"`
    Processing   ProcessingConfig  `yaml:"processing"`
    Security     SecurityConfig    `yaml:"security"`
    CDN          CDNConfig         `yaml:"cdn"`
}

// CacheConfig defines in-memory cache settings
type CacheConfig struct {
    MaxSizeMB      int           `yaml:"max_size_mb"`
    TTLSeconds     int           `yaml:"ttl_seconds"`
    CleanupInterval time.Duration `yaml:"cleanup_interval"`
}

// ProcessingConfig defines image processing settings
type ProcessingConfig struct {
    MaxFileSizeMB    int      `yaml:"max_file_size_mb"`
    AllowedFormats   []string `yaml:"allowed_formats"`
    DefaultQuality   int      `yaml:"default_quality"`
    MaxDimensions    int      `yaml:"max_dimensions"`
    BlurPlaceholder  bool     `yaml:"blur_placeholder"`
}

// CDNConfig defines mock CDN behavior
type CDNConfig struct {
    BaseLatencyMS    int               `yaml:"base_latency_ms"`
    RegionLatencies  map[string]int    `yaml:"region_latencies"`
    FailureRate      float64           `yaml:"failure_rate"`
    CacheHitRate     float64           `yaml:"cache_hit_rate"`
}
```

## API Design

### REST Endpoints

#### Upload Operations
```
POST /api/images/upload
Content-Type: multipart/form-data

Parameters:
- file: image file (required)
- product_id: string (required)
- type: primary|gallery|variant (default: gallery)
- alt_text: string (optional)

Response:
{
  "id": "img_abc123",
  "product_id": "DOG-TOY-001",
  "type": "primary",
  "original_url": "http://localhost:8083/images/img_abc123/original",
  "variants": [
    {
      "size": "thumbnail",
      "url": "http://localhost:8083/images/img_abc123?w=150&h=150&fit=cover",
      "width": 150,
      "height": 150
    }
  ],
  "metadata": {
    "width": 1920,
    "height": 1080,
    "format": "jpeg",
    "file_size": 245760,
    "blur_hash": "LGF5?xYk^6#M@-5c,1J5@[or[Q6."
  }
}
```

#### Batch Upload
```
POST /api/images/batch
Content-Type: application/json

Body:
{
  "product_id": "DOG-TOY-001",
  "images": [
    {
      "url": "https://example.com/image1.jpg",
      "type": "primary",
      "alt_text": "Main product view"
    },
    {
      "url": "https://example.com/image2.jpg",
      "type": "gallery",
      "alt_text": "Side view"
    }
  ]
}

Response:
{
  "batch_id": "batch_xyz789",
  "status": "processing",
  "total_images": 2,
  "processed": 0,
  "failed": 0,
  "results": []
}
```

#### Image Retrieval
```
GET /api/images/{imageId}

Response:
{
  "id": "img_abc123",
  "product_id": "DOG-TOY-001",
  "type": "primary",
  "original_url": "http://localhost:8083/images/img_abc123/original",
  "variants": [...],
  "metadata": {...}
}
```

#### Image Rendering with Transformations
```
GET /api/images/{imageId}/render?w=800&h=600&format=webp&quality=85&fit=cover

Response: Binary image data with appropriate headers
```

#### Product Images
```
GET /api/images/product/{productId}

Response:
{
  "product_id": "DOG-TOY-001",
  "images": [
    {
      "id": "img_abc123",
      "type": "primary",
      "url": "http://localhost:8083/images/img_abc123/original",
      "variants": [...]
    }
  ]
}
```

#### Batch Status
```
GET /api/images/batch/{batchId}/status

Response:
{
  "batch_id": "batch_xyz789",
  "status": "completed",
  "total_images": 2,
  "processed": 2,
  "failed": 0,
  "results": [
    {
      "index": 0,
      "status": "success",
      "image_id": "img_def456"
    },
    {
      "index": 1,
      "status": "success",
      "image_id": "img_ghi789"
    }
  ]
}
```

#### Image Deletion
```
DELETE /api/images/{imageId}

Response:
{
  "deleted": true,
  "variants_deleted": 5
}
```

## Storage Organization

### File System Structure

```
./mock-images/
├── originals/              # Original uploaded images
│   ├── 2024/
│   │   ├── 01/
│   │   │   ├── img_abc123.jpg
│   │   │   └── img_def456.png
│   │   └── 02/
│   └── 2025/
├── processed/              # Processed image variants
│   ├── thumbnails/         # 150x150 thumbnails
│   ├── small/             # 300x300 small images
│   ├── medium/            # 600x600 medium images
│   ├── large/             # 1200x1200 large images
│   └── custom/            # Custom sized images
│       ├── w800_h600_q85/
│       └── w400_h300_q90/
├── metadata/              # JSON metadata files
│   ├── img_abc123.json
│   └── img_def456.json
└── temp/                  # Temporary upload processing
    ├── upload_session_123/
    └── batch_processing/
```

### File Naming Conventions

```
Original: {year}/{month}/{image_id}.{ext}
Processed: {size_category}/{image_id}_{width}x{height}_q{quality}.{format}
Custom: custom/{params_hash}/{image_id}.{format}
Metadata: {image_id}.json
```

## Image Processing Pipeline

### Processing Workflow

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐
│   Upload    │───▶│  Validation  │───▶│   Storage   │
│   Request   │    │  & Security  │    │  Original   │
└─────────────┘    └──────────────┘    └─────────────┘
                                              │
                                              ▼
                   ┌──────────────┐    ┌─────────────┐
                   │   Generate   │◀───│  Metadata   │
                   │  Variants    │    │ Extraction  │
                   └──────────────┘    └─────────────┘
                           │
                           ▼
                   ┌──────────────┐    ┌─────────────┐
                   │    Cache     │───▶│   Return    │
                   │   Results    │    │  Response   │
                   └──────────────┘    └─────────────┘
```

### Processing Operations

#### Image Validation
```go
type ImageValidator struct {
    MaxFileSizeMB   int
    AllowedFormats  []string
    MaxDimensions   int
    VirusScanner    VirusScanner
    ContentScanner  ContentScanner
}

func (v *ImageValidator) Validate(file multipart.File) error {
    // 1. Check file size
    // 2. Validate file format and magic bytes
    // 3. Check image dimensions
    // 4. Scan for malicious content
    // 5. Basic NSFW detection
    return nil
}
```

#### Image Processing
```go
type ImageProcessor struct {
    Config ProcessingConfig
}

func (p *ImageProcessor) ProcessImage(original []byte, req ProcessingRequest) (*ProcessedImage, error) {
    // 1. Decode original image
    // 2. Apply transformations (resize, crop, format conversion)
    // 3. Optimize for web delivery
    // 4. Generate blur hash for lazy loading
    // 5. Extract dominant colors
    // 6. Encode to target format
    return processedImage, nil
}
```

#### Variant Generation
```go
type VariantGenerator struct {
    Processor *ImageProcessor
    Presets   []VariantPreset
}

type VariantPreset struct {
    Name    string
    Width   int
    Height  int
    Format  string
    Quality int
    Fit     string
}

func (g *VariantGenerator) GenerateVariants(original []byte) ([]ImageVariant, error) {
    // Generate standard variants: thumbnail, small, medium, large
    // Apply different formats: WebP, JPEG, AVIF
    // Optimize each variant for its use case
    return variants, nil
}
```

## Mock CDN Implementation (FurryFast CDN)

### CDN Server Architecture

```go
type MockCDN struct {
    Config       CDNConfig
    Cache        *CDNCache
    LatencyMap   map[string]time.Duration
    FailureRate  float64
    Storage      Storage
}

func (cdn *MockCDN) ServeImage(w http.ResponseWriter, r *http.Request) {
    // 1. Simulate geographic latency
    // 2. Simulate cache hit/miss
    // 3. Apply random failures based on configuration
    // 4. Set appropriate cache headers
    // 5. Serve image with simulated CDN behavior
}
```

### CDN Features

#### Geographic Latency Simulation
```go
type LatencySimulator struct {
    RegionLatencies map[string]time.Duration
    BaseLatency     time.Duration
}

func (l *LatencySimulator) SimulateLatency(clientIP string) {
    region := l.detectRegion(clientIP)
    latency := l.RegionLatencies[region]
    if latency == 0 {
        latency = l.BaseLatency
    }
    time.Sleep(latency)
}
```

#### Cache Simulation
```go
type CDNCache struct {
    HitRate    float64
    CachedURLs map[string]CacheEntry
    mutex      sync.RWMutex
}

type CacheEntry struct {
    Data       []byte
    Headers    http.Header
    CachedAt   time.Time
    TTL        time.Duration
}

func (c *CDNCache) Get(url string) (*CacheEntry, bool) {
    // Simulate cache hit/miss based on configured hit rate
    if rand.Float64() > c.HitRate {
        return nil, false // Cache miss
    }
    return entry, true // Cache hit
}
```

#### Failover Simulation
```go
type FailoverManager struct {
    PrimaryURL   string
    BackupURL    string
    FailureRate  float64
    HealthCheck  func() bool
}

func (f *FailoverManager) GetActiveEndpoint() string {
    if rand.Float64() < f.FailureRate {
        return f.BackupURL // Simulate primary failure
    }
    return f.PrimaryURL
}
```

## Caching Strategy

### In-Memory Cache Implementation

```go
type ImageCache struct {
    MaxSizeMB       int
    CurrentSizeMB   int64
    TTL            time.Duration
    CleanupInterval time.Duration
    cache          map[string]*CacheEntry
    lru            *list.List
    mutex          sync.RWMutex
}

type CacheEntry struct {
    Key        string
    Data       []byte
    Size       int64
    CreatedAt  time.Time
    AccessedAt time.Time
    element    *list.Element
}

func (c *ImageCache) Get(key string) ([]byte, bool) {
    c.mutex.RLock()
    defer c.mutex.RUnlock()

    entry, exists := c.cache[key]
    if !exists || time.Since(entry.CreatedAt) > c.TTL {
        return nil, false
    }

    // Update LRU
    entry.AccessedAt = time.Now()
    c.lru.MoveToFront(entry.element)

    return entry.Data, true
}

func (c *ImageCache) Set(key string, data []byte) {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    size := int64(len(data))

    // Evict if necessary
    for c.CurrentSizeMB+size > int64(c.MaxSizeMB*1024*1024) {
        c.evictLRU()
    }

    entry := &CacheEntry{
        Key:        key,
        Data:       data,
        Size:       size,
        CreatedAt:  time.Now(),
        AccessedAt: time.Now(),
    }

    entry.element = c.lru.PushFront(entry)
    c.cache[key] = entry
    c.CurrentSizeMB += size
}
```

### Cache Key Strategy

```go
func GenerateCacheKey(imageID string, params ProcessingRequest) string {
    h := sha256.New()
    h.Write([]byte(fmt.Sprintf("%s_%d_%d_%s_%d_%s",
        imageID, params.Width, params.Height, params.Format, params.Quality, params.Fit)))
    return fmt.Sprintf("img_%x", h.Sum(nil))[:16]
}
```

## Performance Optimization

### Optimization Techniques

#### 1. Lazy Image Processing
```go
func (s *ImageService) GetProcessedImage(imageID string, params ProcessingRequest) ([]byte, error) {
    cacheKey := GenerateCacheKey(imageID, params)

    // Check cache first
    if data, found := s.cache.Get(cacheKey); found {
        return data, nil
    }

    // Process on-demand if not cached
    original, err := s.storage.GetOriginal(imageID)
    if err != nil {
        return nil, err
    }

    processed, err := s.processor.ProcessImage(original, params)
    if err != nil {
        return nil, err
    }

    // Cache result
    s.cache.Set(cacheKey, processed.Data)

    return processed.Data, nil
}
```

#### 2. Batch Processing
```go
type BatchProcessor struct {
    WorkerCount int
    QueueSize   int
    jobs        chan BatchJob
    results     chan BatchResult
}

type BatchJob struct {
    ID        string
    ImageURL  string
    ProductID string
    Type      string
    AltText   string
}

func (b *BatchProcessor) ProcessBatch(jobs []BatchJob) (string, error) {
    batchID := generateBatchID()

    // Start workers
    for i := 0; i < b.WorkerCount; i++ {
        go b.worker()
    }

    // Queue jobs
    for _, job := range jobs {
        b.jobs <- job
    }

    return batchID, nil
}
```

#### 3. Progressive Loading Support
```go
func (s *ImageService) GenerateBlurPlaceholder(imageData []byte) (string, error) {
    // Resize to tiny dimensions (20x20)
    tiny, err := s.processor.Resize(imageData, 20, 20)
    if err != nil {
        return "", err
    }

    // Convert to base64 data URL
    base64Data := base64.StdEncoding.EncodeToString(tiny)
    return fmt.Sprintf("data:image/jpeg;base64,%s", base64Data), nil
}

func (s *ImageService) GenerateBlurHash(imageData []byte) (string, error) {
    // Use blurhash algorithm to generate hash
    return blurhash.Encode(4, 4, imageData)
}
```

## OpenTelemetry Instrumentation

### Tracing Implementation

```go
type TelemetryService struct {
    tracer trace.Tracer
    meter  metric.Meter
}

func (t *TelemetryService) InstrumentImageUpload(ctx context.Context, operation func() error) error {
    ctx, span := t.tracer.Start(ctx, "image.upload")
    defer span.End()

    start := time.Now()
    err := operation()
    duration := time.Since(start)

    // Record metrics
    t.uploadDurationHistogram.Record(ctx, duration.Seconds())

    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        t.uploadErrorCounter.Add(ctx, 1)
    } else {
        t.uploadSuccessCounter.Add(ctx, 1)
    }

    return err
}
```

### Custom Metrics

```go
type ImageMetrics struct {
    UploadDuration    metric.Float64Histogram
    ProcessingDuration metric.Float64Histogram
    CacheHitRate      metric.Float64Gauge
    StorageUsage      metric.Int64Gauge
    RequestsPerSecond metric.Float64Counter
    ErrorRate         metric.Float64Counter
}

func (m *ImageMetrics) RecordUpload(ctx context.Context, duration time.Duration, fileSize int64) {
    m.UploadDuration.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("operation", "upload"),
            attribute.Int64("file_size_bytes", fileSize),
        ))
}

func (m *ImageMetrics) RecordProcessing(ctx context.Context, operation string, duration time.Duration) {
    m.ProcessingDuration.Record(ctx, duration.Seconds(),
        metric.WithAttributes(
            attribute.String("operation", operation),
        ))
}
```

### Logging with Context

```go
func (s *ImageService) logWithTrace(ctx context.Context, level string, message string, fields ...interface{}) {
    span := trace.SpanFromContext(ctx)
    traceID := span.SpanContext().TraceID().String()
    spanID := span.SpanContext().SpanID().String()

    logger := s.logger.WithFields(logrus.Fields{
        "trace_id": traceID,
        "span_id":  spanID,
        "service":  "image-service",
    })

    for i := 0; i < len(fields); i += 2 {
        if i+1 < len(fields) {
            logger = logger.WithField(fmt.Sprintf("%v", fields[i]), fields[i+1])
        }
    }

    switch level {
    case "info":
        logger.Info(message)
    case "error":
        logger.Error(message)
    case "debug":
        logger.Debug(message)
    }
}
```

## Configuration Management

### YAML Configuration

```yaml
# config/images.yaml
images:
  port: 8083
  storage_path: "./mock-images"
  cdn_url: "http://localhost:8083/images"

  cache:
    max_size_mb: 512
    ttl_seconds: 3600
    cleanup_interval: "10m"

  processing:
    max_file_size_mb: 20
    allowed_formats: ["jpeg", "jpg", "png", "webp", "avif"]
    default_quality: 85
    max_dimensions: 4096
    blur_placeholder: true

  security:
    virus_scanning: true
    content_scanning: true
    rate_limit_per_minute: 60
    max_uploads_per_batch: 50

  cdn:
    base_latency_ms: 50
    region_latencies:
      "us-east": 20
      "us-west": 30
      "eu": 80
      "asia": 120
    failure_rate: 0.01
    cache_hit_rate: 0.85

  variants:
    presets:
      - name: "thumbnail"
        width: 150
        height: 150
        format: "webp"
        quality: 80
        fit: "cover"
      - name: "small"
        width: 300
        height: 300
        format: "webp"
        quality: 85
        fit: "cover"
      - name: "medium"
        width: 600
        height: 600
        format: "webp"
        quality: 85
        fit: "inside"
      - name: "large"
        width: 1200
        height: 1200
        format: "webp"
        quality: 90
        fit: "inside"

# Initial image catalog
initial_images:
  - product_id: "DOG-TOY-001"
    type: "primary"
    filename: "rope-toy-lg-1.jpg"
    alt_text: "Large rope toy main view"
  - product_id: "DOG-TOY-001"
    type: "gallery"
    filename: "rope-toy-lg-2.jpg"
    alt_text: "Dog playing with rope toy"
  - product_id: "DOG-TREAT-001"
    type: "primary"
    filename: "bacon-treats-1.jpg"
    alt_text: "Bacon treats package"
```

## Testing Strategy

### Unit Tests

#### Core Service Tests
```go
func TestImageService_Upload(t *testing.T) {
    tests := []struct {
        name     string
        file     multipart.File
        metadata ImageMetadata
        wantErr  bool
    }{
        {
            name: "valid JPEG upload",
            file: createTestImageFile("test.jpg", "image/jpeg"),
            metadata: ImageMetadata{ProductID: "TEST-001", Type: "primary"},
            wantErr: false,
        },
        {
            name: "file too large",
            file: createLargeTestFile(25 * 1024 * 1024), // 25MB
            wantErr: true,
        },
        {
            name: "invalid format",
            file: createTestFile("test.txt", "text/plain"),
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewTestImageService()
            _, err := service.Upload(context.Background(), tt.file, tt.metadata)

            if (err != nil) != tt.wantErr {
                t.Errorf("Upload() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### Processing Tests
```go
func TestImageProcessor_Resize(t *testing.T) {
    processor := NewImageProcessor(ProcessingConfig{
        DefaultQuality: 85,
        MaxDimensions: 4096,
    })

    tests := []struct {
        name      string
        input     ProcessingRequest
        wantWidth int
        wantHeight int
    }{
        {
            name: "resize to thumbnail",
            input: ProcessingRequest{Width: 150, Height: 150, Fit: "cover"},
            wantWidth: 150,
            wantHeight: 150,
        },
        {
            name: "resize maintaining aspect ratio",
            input: ProcessingRequest{Width: 800, Height: 600, Fit: "inside"},
            wantWidth: 800,
            wantHeight: 600,
        },
    }

    original := loadTestImage("test_image.jpg")

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := processor.ProcessImage(original, tt.input)
            require.NoError(t, err)
            assert.Equal(t, tt.wantWidth, result.Width)
            assert.Equal(t, tt.wantHeight, result.Height)
        })
    }
}
```

#### Cache Tests
```go
func TestImageCache_GetSet(t *testing.T) {
    cache := NewImageCache(CacheConfig{
        MaxSizeMB: 10,
        TTLSeconds: 300,
    })

    key := "test_key"
    data := []byte("test image data")

    // Test cache miss
    _, found := cache.Get(key)
    assert.False(t, found)

    // Test cache set and hit
    cache.Set(key, data)
    retrieved, found := cache.Get(key)
    assert.True(t, found)
    assert.Equal(t, data, retrieved)

    // Test TTL expiration
    cache.TTL = time.Millisecond
    time.Sleep(2 * time.Millisecond)
    _, found = cache.Get(key)
    assert.False(t, found)
}
```

### Integration Tests

#### API Integration Tests
```go
func TestImageAPI_UploadAndRetrieve(t *testing.T) {
    server := NewTestServer()
    defer server.Close()

    // Upload image
    uploadResp := uploadTestImage(t, server.URL, "test_product.jpg")
    assert.Equal(t, http.StatusCreated, uploadResp.StatusCode)

    var uploadResult UploadResponse
    json.NewDecoder(uploadResp.Body).Decode(&uploadResult)

    // Retrieve image
    retrieveResp, err := http.Get(fmt.Sprintf("%s/api/images/%s", server.URL, uploadResult.ID))
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, retrieveResp.StatusCode)

    // Test image rendering
    renderURL := fmt.Sprintf("%s/api/images/%s/render?w=300&h=300&format=webp",
        server.URL, uploadResult.ID)
    renderResp, err := http.Get(renderURL)
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, renderResp.StatusCode)
    assert.Equal(t, "image/webp", renderResp.Header.Get("Content-Type"))
}
```

### Performance Tests

#### Load Testing
```go
func TestImageService_LoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test in short mode")
    }

    service := NewTestImageService()
    concurrency := 50
    requestsPerWorker := 100

    var wg sync.WaitGroup
    results := make(chan time.Duration, concurrency*requestsPerWorker)

    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < requestsPerWorker; j++ {
                start := time.Now()
                _, err := service.GetProcessedImage("test_image", ProcessingRequest{
                    Width: 300, Height: 300, Format: "webp",
                })
                duration := time.Since(start)
                results <- duration

                if err != nil {
                    t.Errorf("Request failed: %v", err)
                }
            }
        }()
    }

    wg.Wait()
    close(results)

    // Analyze performance
    var durations []time.Duration
    for duration := range results {
        durations = append(durations, duration)
    }

    sort.Slice(durations, func(i, j int) bool {
        return durations[i] < durations[j]
    })

    p95 := durations[int(float64(len(durations))*0.95)]
    assert.Less(t, p95, 500*time.Millisecond, "P95 latency should be under 500ms")
}
```

#### Memory Usage Tests
```go
func TestImageCache_MemoryUsage(t *testing.T) {
    cache := NewImageCache(CacheConfig{MaxSizeMB: 5})

    // Fill cache with test data
    for i := 0; i < 100; i++ {
        key := fmt.Sprintf("key_%d", i)
        data := make([]byte, 100*1024) // 100KB each
        cache.Set(key, data)
    }

    // Verify cache size limits are respected
    assert.LessOrEqual(t, cache.CurrentSizeMB, int64(5*1024*1024))

    // Test LRU eviction
    cache.Get("key_0") // Make key_0 recently used

    // Add one more item to trigger eviction
    cache.Set("new_key", make([]byte, 200*1024))

    // key_0 should still exist (recently used)
    _, found := cache.Get("key_0")
    assert.True(t, found)

    // Some other key should have been evicted
    evictedCount := 0
    for i := 1; i < 100; i++ {
        key := fmt.Sprintf("key_%d", i)
        _, found := cache.Get(key)
        if !found {
            evictedCount++
        }
    }
    assert.Greater(t, evictedCount, 0)
}
```

## Implementation Order

### Phase 1: Core Infrastructure (Week 1)
1. Set up project structure and configuration
2. Implement mock database layer for image metadata
3. Create basic file storage with organized directory structure
4. Implement image validation and security checks
5. Set up OpenTelemetry instrumentation framework

### Phase 2: Basic Image Operations (Week 1-2)
1. Implement image upload endpoint with validation
2. Create image metadata extraction and storage
3. Implement basic image retrieval by ID
4. Add simple image processing (resize, format conversion)
5. Implement product image listing endpoint

### Phase 3: Processing Pipeline (Week 2)
1. Build comprehensive image processing pipeline
2. Implement variant generation for standard sizes
3. Add on-demand image processing with caching
4. Create blur placeholder and blurhash generation
5. Implement batch processing for multiple images

### Phase 4: Caching and CDN (Week 2-3)
1. Implement in-memory cache with LRU eviction
2. Build mock CDN server with latency simulation
3. Add geographic latency and failure simulation
4. Implement cache invalidation strategies
5. Add CDN failover behavior

### Phase 5: Performance and Optimization (Week 3)
1. Optimize image processing pipeline
2. Implement advanced caching strategies
3. Add progressive loading support
4. Optimize batch processing workflows
5. Implement performance monitoring and alerting

### Phase 6: Testing and Documentation (Week 4)
1. Complete comprehensive unit test suite
2. Implement integration tests for all endpoints
3. Add performance and load testing
4. Create API documentation
5. Add monitoring dashboards and alerts

## Dependencies

### Required Go Packages
```go
// Core dependencies
"github.com/gin-gonic/gin"           // HTTP framework
"github.com/disintegration/imaging"  // Image processing
"github.com/davidbyttow/govips/v2"   // High-performance image processing
"github.com/buckket/go-blurhash"     // Blur hash generation

// Storage and caching
"github.com/patrickmn/go-cache"      // In-memory caching
"github.com/go-redis/redis/v8"       // Redis client (for future use)

// Configuration and logging
"gopkg.in/yaml.v3"                   // YAML configuration
"github.com/sirupsen/logrus"         // Structured logging
"github.com/spf13/viper"            // Configuration management

// OpenTelemetry
"go.opentelemetry.io/otel"
"go.opentelemetry.io/otel/trace"
"go.opentelemetry.io/otel/metric"
"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

// Testing
"github.com/stretchr/testify"        // Testing framework
"github.com/golang/mock"             // Mock generation
```

### External Tools
```bash
# Required for image processing
libvips-dev    # VIPS image processing library
libjpeg-dev    # JPEG support
libpng-dev     # PNG support
libwebp-dev    # WebP support
libavif-dev    # AVIF support (optional)

# Development tools
golangci-lint  # Code linting
mockgen        # Mock generation
go-migrate     # Database migrations (future)
```

## Security Considerations

### File Upload Security
- Magic byte validation for all uploaded files
- File size limits to prevent DoS attacks
- Virus scanning integration (mock implementation)
- Content type validation beyond file extension
- Temporary file cleanup to prevent disk exhaustion

### Image Processing Security
- Input sanitization for all processing parameters
- Resource limits to prevent memory exhaustion
- Timeout limits for processing operations
- Safe handling of EXIF data to prevent information leakage
- Validation of image dimensions to prevent decompression bombs

### API Security
- Rate limiting on upload endpoints
- Authentication token validation
- CORS configuration for web clients
- Request size limits
- SQL injection prevention (even for mock DB)

This comprehensive system design provides a robust foundation for implementing the Image Service component of the Griffin Commerce demo. The design emphasizes simplicity, testability, and observability while simulating real-world CDN behavior and maintaining high performance standards.