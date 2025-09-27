# Recommendations Service System Design

## Overview
The Recommendations Service provides personalized product recommendations for the Griffin Commerce dog product store. It starts with rule-based recommendations and evolves to include simple machine learning, emphasizing explainable recommendations and comprehensive observability.

## System Architecture

### High-Level Components
```
┌─────────────────────────────────────────────────────────────────┐
│                    Recommendations Service                      │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐    │
│ │   API Layer     │ │  Caching Layer  │ │  Config Layer   │    │
│ │   (REST/HTTP)   │ │   (In-Memory)   │ │    (YAML)       │    │
│ └─────────────────┘ └─────────────────┘ └─────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐    │
│ │ Recommendation  │ │   User Profile  │ │  A/B Testing    │    │
│ │    Engine       │ │    Manager      │ │   Framework     │    │
│ └─────────────────┘ └─────────────────┘ └─────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐    │
│ │ Rule-Based      │ │ Product Affinity│ │  Interaction    │    │
│ │ Algorithms      │ │    Calculator   │ │    Tracker      │    │
│ └─────────────────┘ └─────────────────┘ └─────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│ ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐    │
│ │   Mock Database │ │   Telemetry     │ │ Business Rules  │    │
│ │   (In-Memory)   │ │   (OpenTel)     │ │    Engine       │    │
│ └─────────────────┘ └─────────────────┘ └─────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## File Structure
```
recommendations/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── handlers.go
│   │   ├── middleware.go
│   │   └── routes.go
│   ├── algorithms/
│   │   ├── rule_based.go
│   │   ├── collaborative.go
│   │   ├── content_based.go
│   │   └── hybrid.go
│   ├── cache/
│   │   ├── memory_cache.go
│   │   └── cache_interface.go
│   ├── config/
│   │   ├── config.go
│   │   └── affinity_rules.go
│   ├── engine/
│   │   ├── recommendation_engine.go
│   │   ├── user_profile_manager.go
│   │   └── product_affinity_calculator.go
│   ├── models/
│   │   ├── user.go
│   │   ├── product.go
│   │   ├── interaction.go
│   │   ├── recommendation.go
│   │   └── affinity.go
│   ├── mockdb/
│   │   ├── database.go
│   │   ├── user_repository.go
│   │   ├── interaction_repository.go
│   │   └── affinity_repository.go
│   ├── business/
│   │   ├── rules_engine.go
│   │   └── validation.go
│   ├── abtest/
│   │   ├── framework.go
│   │   ├── variants.go
│   │   └── analytics.go
│   └── telemetry/
│       ├── metrics.go
│       ├── tracing.go
│       └── logging.go
├── configs/
│   ├── config.yaml
│   ├── product_affinities.yaml
│   ├── abtest_config.yaml
│   └── mock_data.yaml
├── pkg/
│   └── client/
│       └── recommendations_client.go
└── test/
    ├── integration/
    ├── unit/
    └── testdata/
```

## Data Models

### Core Domain Models

#### User Profile
```go
// UserProfile represents customer preferences and behavior patterns
type UserProfile struct {
    UserID           string                 `json:"user_id"`
    BreedPreference  []string              `json:"breed_preference"`
    SizePreference   []DogSize             `json:"size_preference"`
    AgePreference    []DogAge              `json:"age_preference"`
    PriceRange       PriceRange            `json:"price_range"`
    DietaryNeeds     []DietaryRestriction  `json:"dietary_needs"`
    Preferences      map[string]float64    `json:"preferences"`        // category -> preference score
    InteractionCount int64                 `json:"interaction_count"`
    LastActive       time.Time             `json:"last_active"`
    CreatedAt        time.Time             `json:"created_at"`
    UpdatedAt        time.Time             `json:"updated_at"`
}

type DogSize string
const (
    DogSizeToy    DogSize = "toy"
    DogSizeSmall  DogSize = "small"
    DogSizeMedium DogSize = "medium"
    DogSizeLarge  DogSize = "large"
    DogSizeXLarge DogSize = "xlarge"
)

type DogAge string
const (
    DogAgePuppy  DogAge = "puppy"
    DogAgeAdult  DogAge = "adult"
    DogAgeSenior DogAge = "senior"
)

type PriceRange struct {
    MinPrice float64 `json:"min_price"`
    MaxPrice float64 `json:"max_price"`
}

type DietaryRestriction string
const (
    DietaryGrainFree    DietaryRestriction = "grain_free"
    DietaryChickenFree  DietaryRestriction = "chicken_free"
    DietaryBeefFree     DietaryRestriction = "beef_free"
    DietaryLimitedIngredient DietaryRestriction = "limited_ingredient"
)
```

#### Interaction Event
```go
// InteractionEvent tracks user behavior for recommendation algorithms
type InteractionEvent struct {
    ID            string                 `json:"id"`
    UserID        string                 `json:"user_id"`
    ProductID     string                 `json:"product_id"`
    EventType     InteractionType        `json:"event_type"`
    Timestamp     time.Time              `json:"timestamp"`
    Context       map[string]interface{} `json:"context"`
    SessionID     string                 `json:"session_id"`
    Source        string                 `json:"source"`         // homepage, search, product_page, etc.
    Value         float64                `json:"value"`          // purchase amount, time spent, etc.
}

type InteractionType string
const (
    InteractionView       InteractionType = "view"
    InteractionClick      InteractionType = "click"
    InteractionAddToCart  InteractionType = "add_to_cart"
    InteractionPurchase   InteractionType = "purchase"
    InteractionRemoveCart InteractionType = "remove_from_cart"
    InteractionWishlist   InteractionType = "add_to_wishlist"
    InteractionReview     InteractionType = "review"
    InteractionSearch     InteractionType = "search"
)
```

#### Product Affinity
```go
// ProductAffinity represents relationships between products
type ProductAffinity struct {
    ProductID1       string    `json:"product_id_1"`
    ProductID2       string    `json:"product_id_2"`
    AffinityScore    float64   `json:"affinity_score"`     // 0.0 to 1.0
    CooccurrenceCount int64    `json:"cooccurrence_count"`
    AffinityType     AffinityType `json:"affinity_type"`
    LastUpdated      time.Time `json:"last_updated"`
    Confidence       float64   `json:"confidence"`         // statistical confidence
}

type AffinityType string
const (
    AffinityFrequentlyBought AffinityType = "frequently_bought_together"
    AffinitySimilar          AffinityType = "similar_products"
    AffinityComplementary    AffinityType = "complementary"
    AffinityAlternative      AffinityType = "alternative"
)
```

#### Recommendation
```go
// Recommendation represents a product recommendation for a user
type Recommendation struct {
    ID            string                 `json:"id"`
    UserID        string                 `json:"user_id"`
    ProductID     string                 `json:"product_id"`
    Score         float64                `json:"score"`          // 0.0 to 1.0
    Confidence    float64                `json:"confidence"`     // algorithm confidence
    Algorithm     string                 `json:"algorithm"`
    ReasonCode    ReasonCode             `json:"reason_code"`
    Context       map[string]interface{} `json:"context"`
    CreatedAt     time.Time              `json:"created_at"`
    ExpiresAt     time.Time              `json:"expires_at"`
    ABTestVariant string                 `json:"ab_test_variant"`
    Explanation   string                 `json:"explanation"`    // human-readable reason
}

type ReasonCode string
const (
    ReasonPersonalized        ReasonCode = "personalized"
    ReasonFrequentlyBought   ReasonCode = "frequently_bought_together"
    ReasonSimilarProducts    ReasonCode = "similar_products"
    ReasonTrending           ReasonCode = "trending"
    ReasonNewArrivals        ReasonCode = "new_arrivals"
    ReasonSeasonal           ReasonCode = "seasonal"
    ReasonReorder            ReasonCode = "reorder_suggestion"
    ReasonBundle             ReasonCode = "bundle_suggestion"
    ReasonBreedSpecific      ReasonCode = "breed_specific"
    ReasonSizeAppropriate    ReasonCode = "size_appropriate"
)
```

#### Recommendation Feedback
```go
// RecommendationFeedback tracks user responses to recommendations
type RecommendationFeedback struct {
    ID               string        `json:"id"`
    UserID           string        `json:"user_id"`
    RecommendationID string        `json:"recommendation_id"`
    ProductID        string        `json:"product_id"`
    Action           FeedbackAction `json:"action"`
    Timestamp        time.Time     `json:"timestamp"`
    Context          map[string]interface{} `json:"context"`
}

type FeedbackAction string
const (
    FeedbackClick      FeedbackAction = "click"
    FeedbackView       FeedbackAction = "view"
    FeedbackAddToCart  FeedbackAction = "add_to_cart"
    FeedbackPurchase   FeedbackAction = "purchase"
    FeedbackDismiss    FeedbackAction = "dismiss"
    FeedbackThumbsUp   FeedbackAction = "thumbs_up"
    FeedbackThumbsDown FeedbackAction = "thumbs_down"
)
```

## Component Specifications

### 1. Recommendation Engine

**File: `internal/engine/recommendation_engine.go`**

```go
type RecommendationEngine interface {
    // Generate personalized recommendations for a user
    GetPersonalizedRecommendations(ctx context.Context, userID string, limit int, context map[string]interface{}) ([]Recommendation, error)

    // Get similar products for a given product
    GetSimilarProducts(ctx context.Context, productID string, limit int) ([]Recommendation, error)

    // Get frequently bought together recommendations
    GetFrequentlyBoughtTogether(ctx context.Context, productID string, limit int) ([]Recommendation, error)

    // Get trending products by category
    GetTrendingProducts(ctx context.Context, category string, limit int) ([]Recommendation, error)

    // Get cart-based recommendations
    GetCartRecommendations(ctx context.Context, cartID string, limit int) ([]Recommendation, error)

    // Get reorder recommendations
    GetReorderRecommendations(ctx context.Context, userID string, limit int) ([]Recommendation, error)

    // Record user interaction for future recommendations
    RecordInteraction(ctx context.Context, event InteractionEvent) error

    // Update user preferences based on feedback
    UpdateUserPreferences(ctx context.Context, userID string, preferences UserProfile) error
}

type DefaultRecommendationEngine struct {
    userProfileManager     UserProfileManager
    productAffinityCalc    ProductAffinityCalculator
    algorithms            map[string]RecommendationAlgorithm
    businessRules         BusinessRulesEngine
    cache                 cache.Cache
    abTestFramework       abtest.Framework
    telemetry             telemetry.Service
    config                *config.Config
}
```

**Responsibilities:**
- Orchestrate different recommendation algorithms
- Apply business rules and filters
- Manage caching of recommendations
- Handle A/B testing for different algorithms
- Provide explainable recommendations
- Track performance metrics

### 2. User Profile Manager

**File: `internal/engine/user_profile_manager.go`**

```go
type UserProfileManager interface {
    // Get or create user profile
    GetProfile(ctx context.Context, userID string) (*UserProfile, error)

    // Update user profile with new interaction
    UpdateProfileFromInteraction(ctx context.Context, userID string, event InteractionEvent) error

    // Update explicit preferences
    UpdateExplicitPreferences(ctx context.Context, userID string, preferences map[string]interface{}) error

    // Calculate user similarity for collaborative filtering
    CalculateUserSimilarity(ctx context.Context, userID1, userID2 string) (float64, error)

    // Get users with similar preferences
    GetSimilarUsers(ctx context.Context, userID string, limit int) ([]string, error)

    // Handle cold start for new users
    HandleColdStart(ctx context.Context, userID string, initialContext map[string]interface{}) (*UserProfile, error)
}

type DefaultUserProfileManager struct {
    userRepo      mockdb.UserRepository
    interactionRepo mockdb.InteractionRepository
    cache         cache.Cache
    telemetry     telemetry.Service
    coldStartRules map[string]interface{}
}
```

**Responsibilities:**
- Maintain user preference profiles
- Learn from user interactions
- Handle cold start scenarios
- Calculate user similarities
- Update profiles in real-time

### 3. Product Affinity Calculator

**File: `internal/engine/product_affinity_calculator.go`**

```go
type ProductAffinityCalculator interface {
    // Calculate affinity between two products
    CalculateAffinity(ctx context.Context, productID1, productID2 string) (float64, error)

    // Get products with highest affinity to given product
    GetMostAffinityProducts(ctx context.Context, productID string, affinityType AffinityType, limit int) ([]ProductAffinity, error)

    // Update affinity scores based on new interactions
    UpdateAffinityScores(ctx context.Context, events []InteractionEvent) error

    // Batch calculate affinities for multiple products
    BatchCalculateAffinities(ctx context.Context, productIDs []string) error

    // Get product embeddings for similarity calculation
    GetProductEmbedding(ctx context.Context, productID string) ([]float64, error)
}

type DefaultProductAffinityCalculator struct {
    affinityRepo    mockdb.AffinityRepository
    interactionRepo mockdb.InteractionRepository
    productRepo     mockdb.ProductRepository
    cache           cache.Cache
    telemetry       telemetry.Service
    config          *config.AffinityConfig
}
```

**Responsibilities:**
- Calculate product-to-product relationships
- Maintain affinity scores
- Support different affinity types
- Update scores based on user behavior
- Provide fast lookups for similar products

### 4. Rule-Based Algorithm (Initial Implementation)

**File: `internal/algorithms/rule_based.go`**

```go
type RuleBasedAlgorithm struct {
    userProfileManager UserProfileManager
    productAffinityCalc ProductAffinityCalculator
    businessRules      BusinessRulesEngine
    config            *config.RuleBasedConfig
    telemetry         telemetry.Service
}

type RuleBasedConfig struct {
    Weights struct {
        CategoryPreference  float64 `yaml:"category_preference"`
        PriceRange         float64 `yaml:"price_range"`
        BrandLoyalty       float64 `yaml:"brand_loyalty"`
        SizeCompatibility  float64 `yaml:"size_compatibility"`
        RecentInteractions float64 `yaml:"recent_interactions"`
        PopularityBoost    float64 `yaml:"popularity_boost"`
    } `yaml:"weights"`

    Thresholds struct {
        MinScore           float64 `yaml:"min_score"`
        MinConfidence      float64 `yaml:"min_confidence"`
        MaxAge             string  `yaml:"max_age"`
        MinInteractions    int     `yaml:"min_interactions"`
    } `yaml:"thresholds"`
}

func (r *RuleBasedAlgorithm) GenerateRecommendations(ctx context.Context, userID string, context map[string]interface{}) ([]Recommendation, error) {
    // 1. Get user profile
    // 2. Apply category preferences
    // 3. Filter by size/age appropriateness
    // 4. Apply price range filters
    // 5. Boost popular items
    // 6. Score and rank recommendations
    // 7. Apply business rules
    // 8. Return top recommendations with explanations
}
```

**Scoring Algorithm:**
1. **Category Preference**: Score based on user's historical category interactions
2. **Price Range Fit**: Score based on how well product fits user's price range
3. **Size/Age Appropriateness**: Boost products appropriate for user's dog
4. **Recency Boost**: Higher scores for recently viewed/purchased categories
5. **Popularity Factor**: Slight boost for generally popular products
6. **Diversity Factor**: Ensure recommendations span multiple categories

### 5. Business Rules Engine

**File: `internal/business/rules_engine.go`**

```go
type BusinessRulesEngine interface {
    // Apply business rules to filter recommendations
    ApplyRules(ctx context.Context, userID string, recommendations []Recommendation) ([]Recommendation, error)

    // Validate recommendation against business constraints
    ValidateRecommendation(ctx context.Context, userID string, productID string) error

    // Get explanation for why a recommendation was filtered out
    GetFilterReason(ctx context.Context, userID string, productID string) (string, error)
}

type DefaultBusinessRulesEngine struct {
    productRepo mockdb.ProductRepository
    userRepo    mockdb.UserRepository
    config      *config.BusinessRulesConfig
    telemetry   telemetry.Service
}

type BusinessRule interface {
    Apply(ctx context.Context, userID string, recommendations []Recommendation) ([]Recommendation, error)
    Name() string
    Priority() int
}

// Specific business rules
type InventoryRule struct{}        // Filter out-of-stock items
type PriceRangeRule struct{}      // Respect user price preferences
type AgeAppropriateRule struct{}  // Age-appropriate recommendations
type SizeAppropriateRule struct{} // Size-appropriate recommendations
type DietaryRestrictionsRule struct{} // Dietary restrictions
type MinimumScoreRule struct{}    // Minimum confidence threshold
```

**Business Rules:**
1. **Inventory Rule**: Never recommend out-of-stock products
2. **Price Range Rule**: Filter products outside user's price range
3. **Age Appropriate Rule**: Ensure puppy/senior appropriate products
4. **Size Appropriate Rule**: Match product size to dog size
5. **Dietary Restrictions Rule**: Respect food allergies/preferences
6. **Minimum Score Rule**: Only show recommendations above confidence threshold
7. **Diversity Rule**: Ensure variety in recommendations
8. **Recency Rule**: Don't recommend recently purchased items (except consumables)

### 6. A/B Testing Framework

**File: `internal/abtest/framework.go`**

```go
type Framework interface {
    // Get variant assignment for user
    GetVariant(ctx context.Context, userID, experimentName string) (string, error)

    // Record conversion event
    RecordConversion(ctx context.Context, userID, experimentName, variant string, conversionType string, value float64) error

    // Get experiment results
    GetExperimentResults(ctx context.Context, experimentName string) (*ExperimentResults, error)

    // Create new experiment
    CreateExperiment(ctx context.Context, experiment Experiment) error
}

type Experiment struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Variants    []Variant             `json:"variants"`
    TrafficSplit map[string]float64   `json:"traffic_split"`
    StartDate   time.Time             `json:"start_date"`
    EndDate     time.Time             `json:"end_date"`
    Status      ExperimentStatus      `json:"status"`
    Metrics     []string              `json:"metrics"`
}

type Variant struct {
    Name        string                 `json:"name"`
    Config      map[string]interface{} `json:"config"`
    Description string                 `json:"description"`
}

// Example A/B tests for recommendations
var DefaultExperiments = []Experiment{
    {
        Name: "recommendation_algorithm",
        Variants: []Variant{
            {Name: "rule_based", Config: map[string]interface{}{"algorithm": "rule_based"}},
            {Name: "collaborative", Config: map[string]interface{}{"algorithm": "collaborative_filtering"}},
            {Name: "hybrid", Config: map[string]interface{}{"algorithm": "hybrid"}},
        },
        TrafficSplit: map[string]float64{
            "rule_based": 0.5,
            "collaborative": 0.25,
            "hybrid": 0.25,
        },
    },
    {
        Name: "recommendation_count",
        Variants: []Variant{
            {Name: "four_recs", Config: map[string]interface{}{"count": 4}},
            {Name: "six_recs", Config: map[string]interface{}{"count": 6}},
            {Name: "eight_recs", Config: map[string]interface{}{"count": 8}},
        },
        TrafficSplit: map[string]float64{
            "four_recs": 0.33,
            "six_recs": 0.34,
            "eight_recs": 0.33,
        },
    },
}
```

**A/B Testing Scenarios:**
1. **Algorithm Comparison**: Rule-based vs. Collaborative vs. Hybrid
2. **Recommendation Count**: 4 vs. 6 vs. 8 recommendations
3. **Explanation Display**: With vs. without explanations
4. **Layout Testing**: Horizontal vs. vertical recommendation layout
5. **Scoring Weights**: Different weight combinations for rule-based scoring

### 7. Caching Strategy

**File: `internal/cache/memory_cache.go`**

```go
type Cache interface {
    Get(ctx context.Context, key string) (interface{}, error)
    Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    GetStats() CacheStats
}

type MemoryCache struct {
    store     sync.Map
    ttlStore  sync.Map
    stats     CacheStats
    mutex     sync.RWMutex
    janitor   *time.Ticker
    telemetry telemetry.Service
}

type CacheKey struct {
    Type   string
    UserID string
    Context string
}

// Cache key patterns
const (
    PersonalizedRecsKey    = "recs:personal:%s:%s"      // userID, context hash
    SimilarProductsKey     = "recs:similar:%s"          // productID
    FrequentlyBoughtKey    = "recs:freq_bought:%s"      // productID
    TrendingProductsKey    = "recs:trending:%s"         // category
    UserProfileKey         = "profile:%s"               // userID
    ProductAffinityKey     = "affinity:%s:%s"          // productID1, productID2
    ABTestVariantKey       = "abtest:%s:%s"            // userID, experimentName
)

// Cache TTL configuration
var CacheTTLs = map[string]time.Duration{
    "personalized":      1 * time.Hour,
    "similar":           24 * time.Hour,
    "frequently_bought": 12 * time.Hour,
    "trending":          30 * time.Minute,
    "user_profile":      6 * time.Hour,
    "affinity":          24 * time.Hour,
    "abtest_variant":    24 * time.Hour,
}
```

**Caching Strategy:**
1. **Personalized Recommendations**: Cache for 1 hour per user/context
2. **Product Similarities**: Cache for 24 hours (updated daily)
3. **User Profiles**: Cache for 6 hours (updated on interactions)
4. **Product Affinities**: Cache for 24 hours (batch updated)
5. **Trending Products**: Cache for 30 minutes (frequently updated)
6. **A/B Test Assignments**: Cache for 24 hours (consistent experience)

## API Design

### REST Endpoints

#### 1. Personalized Recommendations
```http
GET /api/recommendations/user/{userId}
```

**Query Parameters:**
- `limit` (int, default: 6): Number of recommendations to return
- `context` (string): Context for recommendations (homepage, product_page, cart, etc.)
- `category` (string): Filter by product category
- `exclude` ([]string): Product IDs to exclude from recommendations

**Response:**
```json
{
  "user_id": "user123",
  "recommendations": [
    {
      "id": "rec_456",
      "product_id": "DOG-TOY-001",
      "score": 0.89,
      "confidence": 0.92,
      "algorithm": "rule_based",
      "reason_code": "personalized",
      "explanation": "Based on your preference for large dog toys",
      "ab_test_variant": "rule_based",
      "created_at": "2024-01-15T10:30:00Z",
      "expires_at": "2024-01-15T11:30:00Z"
    }
  ],
  "metadata": {
    "total_available": 25,
    "response_time_ms": 45,
    "cache_hit": true,
    "ab_test_experiment": "recommendation_algorithm"
  }
}
```

#### 2. Similar Products
```http
GET /api/recommendations/product/{productId}/similar
```

**Query Parameters:**
- `limit` (int, default: 4): Number of similar products to return
- `affinity_type` (string): Type of similarity (similar_products, alternative)

**Response:**
```json
{
  "product_id": "DOG-TOY-001",
  "similar_products": [
    {
      "product_id": "DOG-TOY-002",
      "affinity_score": 0.87,
      "affinity_type": "similar_products",
      "reason": "Similar size and material",
      "confidence": 0.91
    }
  ],
  "metadata": {
    "algorithm": "content_based",
    "response_time_ms": 23
  }
}
```

#### 3. Frequently Bought Together
```http
GET /api/recommendations/product/{productId}/frequently-bought
```

**Response:**
```json
{
  "product_id": "DOG-TOY-001",
  "frequently_bought_together": [
    {
      "product_id": "DOG-TREAT-001",
      "affinity_score": 0.75,
      "cooccurrence_count": 145,
      "confidence": 0.88,
      "reason": "Purchased together in 32% of orders"
    }
  ]
}
```

#### 4. Trending Products
```http
GET /api/recommendations/trending
```

**Query Parameters:**
- `category` (string): Filter by category
- `time_window` (string): Time window for trending calculation (24h, 7d, 30d)
- `limit` (int, default: 10)

**Response:**
```json
{
  "trending_products": [
    {
      "product_id": "DOG-FOOD-001",
      "trend_score": 0.94,
      "rank_change": 3,
      "interaction_count": 1247,
      "time_window": "24h"
    }
  ]
}
```

#### 5. Cart-Based Recommendations
```http
GET /api/recommendations/cart/{cartId}
```

**Response:**
```json
{
  "cart_id": "cart123",
  "recommendations": [
    {
      "product_id": "DOG-LEASH-001",
      "score": 0.83,
      "reason_code": "complementary",
      "explanation": "Great with the collar in your cart",
      "bundle_discount": 0.15
    }
  ]
}
```

#### 6. Record Feedback
```http
POST /api/recommendations/feedback
```

**Request Body:**
```json
{
  "user_id": "user123",
  "recommendation_id": "rec_456",
  "product_id": "DOG-TOY-001",
  "action": "click",
  "context": {
    "page": "homepage",
    "position": 1
  }
}
```

#### 7. Reorder Recommendations
```http
GET /api/recommendations/reorder/{userId}
```

**Response:**
```json
{
  "user_id": "user123",
  "reorder_suggestions": [
    {
      "product_id": "DOG-FOOD-001",
      "last_purchased": "2024-01-01T00:00:00Z",
      "avg_reorder_interval": "30d",
      "urgency_score": 0.89,
      "explanation": "You typically reorder this every 30 days"
    }
  ]
}
```

## OpenTelemetry Instrumentation

### Metrics Collection

**File: `internal/telemetry/metrics.go`**

```go
type Metrics struct {
    // Recommendation performance
    RecommendationLatency    metric.Float64Histogram
    RecommendationCacheHits  metric.Int64Counter
    RecommendationCacheMiss  metric.Int64Counter

    // Algorithm performance
    AlgorithmExecutionTime   metric.Float64Histogram
    AlgorithmScore          metric.Float64Histogram
    AlgorithmConfidence     metric.Float64Histogram

    // Business metrics
    ClickThroughRate        metric.Float64Gauge
    ConversionRate          metric.Float64Gauge
    RevenueAttribution      metric.Float64Counter

    // User behavior
    UserInteractions        metric.Int64Counter
    ProfileUpdates          metric.Int64Counter
    ColdStartEvents         metric.Int64Counter

    // A/B testing
    ExperimentAssignments   metric.Int64Counter
    ConversionsByVariant    metric.Int64Counter

    // System health
    DatabaseOperations      metric.Int64Counter
    ErrorRate              metric.Int64Counter
}

// Metric labels
var (
    AlgorithmLabel     = "algorithm"
    ReasonCodeLabel    = "reason_code"
    CacheTypeLabel     = "cache_type"
    ABTestVariantLabel = "ab_test_variant"
    ErrorTypeLabel     = "error_type"
)
```

### Tracing Strategy

**Custom Spans:**
1. **recommendation.generate**: Full recommendation generation process
2. **algorithm.execute**: Individual algorithm execution
3. **profile.lookup**: User profile retrieval and updates
4. **affinity.calculate**: Product affinity calculations
5. **business_rules.apply**: Business rule application
6. **cache.operation**: Cache read/write operations
7. **abtest.assign**: A/B test variant assignment

**Trace Attributes:**
- `user.id`: User identifier
- `product.id`: Product identifier
- `algorithm.name`: Algorithm used
- `recommendation.count`: Number of recommendations generated
- `cache.hit`: Whether recommendation was served from cache
- `ab_test.experiment`: A/B test experiment name
- `ab_test.variant`: A/B test variant assigned

### Logging Standards

**Structured Logging Examples:**
```go
// Recommendation generation
logger.InfoContext(ctx, "Generating personalized recommendations",
    "user_id", userID,
    "algorithm", "rule_based",
    "context", requestContext,
    "limit", limit,
    "trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID(),
)

// A/B test assignment
logger.InfoContext(ctx, "A/B test variant assigned",
    "user_id", userID,
    "experiment", "recommendation_algorithm",
    "variant", "collaborative",
    "assignment_reason", "traffic_split",
)

// Performance warning
logger.WarnContext(ctx, "Recommendation generation slow",
    "user_id", userID,
    "algorithm", "collaborative",
    "duration_ms", duration.Milliseconds(),
    "threshold_ms", 200,
)
```

## Testing Strategy

### Unit Testing

**Test Categories:**

1. **Algorithm Testing**
   - Rule-based scoring accuracy
   - Collaborative filtering recommendations
   - Content-based similarity calculations
   - Hybrid algorithm weighting

2. **Business Rules Testing**
   - Inventory filtering
   - Price range validation
   - Age/size appropriateness
   - Dietary restrictions

3. **User Profile Testing**
   - Profile creation and updates
   - Preference learning
   - Cold start handling
   - Similarity calculations

4. **Caching Testing**
   - Cache hit/miss scenarios
   - TTL expiration
   - Cache invalidation
   - Performance under load

**Test Files:**
```
test/unit/
├── algorithms/
│   ├── rule_based_test.go
│   ├── collaborative_test.go
│   └── hybrid_test.go
├── business/
│   ├── rules_engine_test.go
│   └── validation_test.go
├── engine/
│   ├── recommendation_engine_test.go
│   ├── user_profile_manager_test.go
│   └── product_affinity_calculator_test.go
├── cache/
│   └── memory_cache_test.go
└── abtest/
    ├── framework_test.go
    └── analytics_test.go
```

### Integration Testing

**Test Scenarios:**

1. **End-to-End Recommendation Flow**
   - User interaction → Profile update → Recommendation generation
   - A/B test assignment consistency
   - Cache behavior across requests

2. **Algorithm Performance Comparison**
   - Compare different algorithms on same dataset
   - Measure recommendation quality metrics
   - Validate business rule application

3. **Load Testing**
   - Concurrent recommendation requests
   - Cache performance under load
   - Database operation scaling

### Recommendation Quality Metrics

**File: `test/metrics/recommendation_quality.go`**

```go
type QualityMetrics struct {
    // Accuracy metrics
    Precision           float64 // Relevant recommendations / Total recommendations
    Recall              float64 // Relevant recommendations / Total relevant items
    F1Score             float64 // Harmonic mean of precision and recall

    // Diversity metrics
    IntraListDiversity  float64 // Diversity within recommendation list
    Coverage            float64 // Percentage of catalog covered in recommendations
    Novelty             float64 // How novel are the recommendations

    // Business metrics
    ClickThroughRate    float64 // CTR on recommendations
    ConversionRate      float64 // Purchase rate from recommendations
    RevenuePerRec       float64 // Average revenue per recommendation

    // User satisfaction
    ExplanationQuality  float64 // Quality of recommendation explanations
    UserSatisfaction    float64 // User feedback scores
}

func CalculateQualityMetrics(ctx context.Context, testData []TestCase) (*QualityMetrics, error) {
    // Implementation for calculating recommendation quality
}

type TestCase struct {
    UserID              string
    ExpectedRecommendations []string
    ActualRecommendations   []Recommendation
    UserFeedback        []RecommendationFeedback
}
```

### A/B Testing Metrics

```go
type ABTestMetrics struct {
    VariantPerformance map[string]VariantStats
    StatisticalSignificance float64
    ConfidenceInterval     [2]float64
    SampleSize            map[string]int
    TestDuration          time.Duration
}

type VariantStats struct {
    ClickThroughRate   float64
    ConversionRate     float64
    RevenuePerUser     float64
    UserSatisfaction   float64
    RecommendationDiversity float64
}
```

## Configuration Files

### Main Configuration
**File: `configs/config.yaml`**

```yaml
app:
  name: "Griffin Commerce Recommendations"
  port: 8085
  environment: "poc"
  log_level: "debug"

recommendation_engine:
  default_algorithm: "rule_based"
  default_limit: 6
  cache_ttl_hours: 1
  min_confidence_threshold: 0.3
  max_recommendations: 20
  enable_explanations: true

algorithms:
  rule_based:
    enabled: true
    weights:
      category_preference: 0.3
      price_range: 0.2
      brand_loyalty: 0.15
      size_compatibility: 0.15
      recent_interactions: 0.1
      popularity_boost: 0.1
    thresholds:
      min_score: 0.3
      min_confidence: 0.5
      max_age: "7d"
      min_interactions: 1

  collaborative:
    enabled: true
    min_similar_users: 5
    similarity_threshold: 0.4
    neighbor_count: 20

  content_based:
    enabled: true
    feature_weights:
      category: 0.4
      brand: 0.2
      price_range: 0.2
      attributes: 0.2

business_rules:
  inventory:
    exclude_out_of_stock: true
    min_inventory_threshold: 1

  price:
    respect_user_range: true
    max_price_deviation: 0.5  # 50% above user's typical range

  appropriateness:
    enforce_age_restrictions: true
    enforce_size_restrictions: true
    enforce_dietary_restrictions: true

  diversity:
    max_same_category: 3
    min_categories: 2
    ensure_price_diversity: true

cache:
  enabled: true
  type: "memory"
  max_size_mb: 100
  cleanup_interval_minutes: 10
  default_ttl_hours: 1

telemetry:
  enabled: true
  service_name: "recommendations-service"
  otlp_endpoint: "localhost:4317"
  sampling_rate: 1.0
  metrics_port: 9090

mock_db:
  enabled: true
  persist_to_file: true
  data_dir: "./mock-data/recommendations"
  initial_data_file: "mock_data.yaml"

ab_testing:
  enabled: true
  config_file: "abtest_config.yaml"
  assignment_salt: "griffin_recommendations"
```

### Product Affinities Configuration
**File: `configs/product_affinities.yaml`**

```yaml
# Manually configured product affinities for initial POC
affinities:
  frequently_bought_together:
    "DOG-COLLAR-001":
      - product_id: "DOG-LEASH-001"
        score: 0.85
        reason: "Collar and leash are commonly bought as a set"
      - product_id: "DOG-TAG-001"
        score: 0.75
        reason: "Pet identification goes with collars"

    "DOG-FOOD-001":
      - product_id: "DOG-BOWL-001"
        score: 0.70
        reason: "Food and bowls are essential together"
      - product_id: "DOG-TREAT-001"
        score: 0.60
        reason: "Treats complement main meals"

    "DOG-TOY-ROPE-001":
      - product_id: "DOG-TREAT-TRAINING-001"
        score: 0.65
        reason: "Training treats enhance interactive play"

  similar_products:
    "DOG-TOY-ROPE-001":
      - product_id: "DOG-TOY-BALL-001"
        score: 0.80
        reason: "Both are interactive toys for active dogs"
      - product_id: "DOG-TOY-SQUEAKY-001"
        score: 0.70
        reason: "Similar engagement level and play style"

  complementary:
    "DOG-SHAMPOO-001":
      - product_id: "DOG-BRUSH-001"
        score: 0.90
        reason: "Grooming products work best together"
      - product_id: "DOG-TOWEL-001"
        score: 0.75
        reason: "Essential for post-bath care"

# Category-based affinity rules
category_affinities:
  toys:
    frequently_with: ["treats", "training"]
    score_multiplier: 0.6

  food:
    frequently_with: ["bowls", "treats", "supplements"]
    score_multiplier: 0.7

  grooming:
    frequently_with: ["health", "accessories"]
    score_multiplier: 0.8

# Seasonal affinity boosts
seasonal_boosts:
  winter:
    categories: ["coats", "boots", "indoor_toys"]
    multiplier: 1.2

  summer:
    categories: ["cooling_mats", "water_toys", "travel"]
    multiplier: 1.3

  holidays:
    christmas:
      products: ["DOG-TOY-HOLIDAY-001", "DOG-TREAT-HOLIDAY-001"]
      multiplier: 1.5
```

### A/B Testing Configuration
**File: `configs/abtest_config.yaml`**

```yaml
experiments:
  recommendation_algorithm:
    name: "Recommendation Algorithm Comparison"
    description: "Compare rule-based vs collaborative filtering"
    status: "active"
    start_date: "2024-01-01T00:00:00Z"
    end_date: "2024-02-01T00:00:00Z"
    traffic_allocation: 1.0  # 100% of users

    variants:
      control:
        name: "Rule-Based Algorithm"
        traffic_split: 0.5
        config:
          algorithm: "rule_based"
          explanation_style: "simple"

      treatment:
        name: "Collaborative Filtering"
        traffic_split: 0.5
        config:
          algorithm: "collaborative"
          explanation_style: "detailed"

    metrics:
      primary: "click_through_rate"
      secondary: ["conversion_rate", "revenue_per_user"]

    success_criteria:
      min_improvement: 0.05  # 5% improvement required
      confidence_level: 0.95
      min_sample_size: 1000

  recommendation_count:
    name: "Optimal Recommendation Count"
    description: "Test different numbers of recommendations"
    status: "active"

    variants:
      four_recs:
        traffic_split: 0.33
        config:
          recommendation_count: 4

      six_recs:
        traffic_split: 0.34
        config:
          recommendation_count: 6

      eight_recs:
        traffic_split: 0.33
        config:
          recommendation_count: 8

  explanation_display:
    name: "Recommendation Explanations"
    description: "Impact of showing recommendation explanations"
    status: "planning"

    variants:
      no_explanation:
        traffic_split: 0.5
        config:
          show_explanations: false

      with_explanation:
        traffic_split: 0.5
        config:
          show_explanations: true
          explanation_style: "user_friendly"

# A/B test assignment configuration
assignment:
  hash_function: "murmur3"
  salt: "griffin_commerce_recommendations_2024"
  sticky_assignment: true  # Users get same variant throughout experiment

# Monitoring and alerting
monitoring:
  auto_stop_criteria:
    max_error_rate: 0.05  # Stop if error rate > 5%
    min_conversion_rate: 0.01  # Stop if conversion < 1%

  alert_thresholds:
    significant_difference: 0.95  # Alert when 95% confidence reached
    sample_size_target: 10000    # Alert when target reached
```

### Mock Data Configuration
**File: `configs/mock_data.yaml`**

```yaml
users:
  - id: "user_001"
    breed_preference: ["labrador", "golden_retriever"]
    size_preference: ["large"]
    age_preference: ["adult"]
    price_range:
      min_price: 10.0
      max_price: 100.0
    dietary_needs: ["grain_free"]
    preferences:
      toys: 0.8
      treats: 0.9
      food: 0.7
      grooming: 0.3
    interaction_count: 45

  - id: "user_002"
    breed_preference: ["chihuahua", "yorkshire_terrier"]
    size_preference: ["toy", "small"]
    age_preference: ["puppy"]
    price_range:
      min_price: 5.0
      max_price: 50.0
    dietary_needs: []
    preferences:
      toys: 0.6
      treats: 0.8
      food: 0.9
      training: 0.7
    interaction_count: 23

interactions:
  - user_id: "user_001"
    product_id: "DOG-TOY-001"
    event_type: "view"
    timestamp: "2024-01-15T10:00:00Z"
    context:
      source: "homepage"
      position: 1

  - user_id: "user_001"
    product_id: "DOG-TOY-001"
    event_type: "add_to_cart"
    timestamp: "2024-01-15T10:05:00Z"
    context:
      source: "product_page"

  - user_id: "user_001"
    product_id: "DOG-TOY-001"
    event_type: "purchase"
    timestamp: "2024-01-15T10:15:00Z"
    value: 15.99

products:
  - id: "DOG-TOY-001"
    category: "toys"
    sub_category: "rope_toys"
    brand: "PuppyPlay"
    price: 15.99
    target_size: ["medium", "large"]
    target_age: ["adult"]
    attributes:
      material: "cotton"
      durability: "high"
      interactive: true
    popularity_score: 0.85
    inventory: 100

  - id: "DOG-TREAT-001"
    category: "treats"
    sub_category: "training_treats"
    brand: "GoodDog"
    price: 9.99
    target_size: ["all"]
    target_age: ["puppy", "adult"]
    attributes:
      flavor: "bacon"
      size: "small"
      training_suitable: true
    popularity_score: 0.92
    inventory: 250

trending_data:
  toys:
    - product_id: "DOG-TOY-001"
      trend_score: 0.94
      rank: 1
      interaction_count_24h: 156
    - product_id: "DOG-TOY-002"
      trend_score: 0.87
      rank: 2
      interaction_count_24h: 134

  treats:
    - product_id: "DOG-TREAT-001"
      trend_score: 0.91
      rank: 1
      interaction_count_24h: 203
```

## Implementation Order

### Phase 1: Core Infrastructure (Week 1)
1. **Set up project structure** and dependency management
2. **Implement mock database layer** with in-memory storage
3. **Create basic configuration system** with YAML loading
4. **Set up OpenTelemetry instrumentation** framework
5. **Implement basic HTTP server** with health endpoints

### Phase 2: Data Models and Storage (Week 2)
1. **Define core data models** (UserProfile, Interaction, Recommendation, etc.)
2. **Implement mock repositories** for all data types
3. **Create initial mock data** for testing
4. **Implement basic caching layer** with in-memory cache
5. **Add data validation** and business rules

### Phase 3: Rule-Based Algorithm (Week 3)
1. **Implement UserProfileManager** with basic preference tracking
2. **Create rule-based recommendation algorithm** with configurable weights
3. **Implement ProductAffinityCalculator** with manual affinity data
4. **Add business rules engine** with core filtering rules
5. **Create basic RecommendationEngine** orchestration

### Phase 4: API Layer (Week 4)
1. **Implement REST API handlers** for all endpoints
2. **Add request/response validation** and error handling
3. **Integrate caching** into API responses
4. **Add rate limiting** and middleware
5. **Implement recommendation explanations**

### Phase 5: A/B Testing Framework (Week 5)
1. **Create A/B testing framework** with variant assignment
2. **Implement experiment configuration** loading
3. **Add conversion tracking** and analytics
4. **Integrate A/B testing** into recommendation generation
5. **Create experiment monitoring** dashboard endpoints

### Phase 6: Advanced Features (Week 6)
1. **Implement collaborative filtering** algorithm (simple version)
2. **Add content-based filtering** using product attributes
3. **Create hybrid algorithm** combining multiple approaches
4. **Implement trending calculation** with time-based scoring
5. **Add reorder recommendations** for consumable products

### Phase 7: Testing and Optimization (Week 7)
1. **Write comprehensive unit tests** for all components
2. **Create integration tests** for API endpoints
3. **Implement recommendation quality metrics** and evaluation
4. **Add performance testing** and optimization
5. **Create load testing** scenarios

### Phase 8: Documentation and Deployment (Week 8)
1. **Complete API documentation** with examples
2. **Create operational runbooks** and troubleshooting guides
3. **Implement monitoring dashboards** and alerts
4. **Package for deployment** with Docker containers
5. **Create demo scenarios** and user journey tests

## Dependencies

### External Libraries
```go
// Core dependencies
"github.com/gin-gonic/gin"                    // HTTP router
"github.com/spf13/viper"                      // Configuration management
"go.uber.org/zap"                             // Structured logging
"github.com/prometheus/client_golang"          // Metrics collection

// OpenTelemetry
"go.opentelemetry.io/otel"                    // Core OpenTelemetry
"go.opentelemetry.io/otel/trace"              // Tracing
"go.opentelemetry.io/otel/metric"             // Metrics
"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

// Utilities
"github.com/google/uuid"                      // UUID generation
"golang.org/x/sync/errgroup"                  // Concurrent operations
"github.com/patrickmn/go-cache"               // In-memory caching
"gopkg.in/yaml.v3"                           // YAML parsing

// Testing
"github.com/stretchr/testify"                 // Test assertions
"github.com/golang/mock"                      // Mock generation
```

### Internal Dependencies
- Product Service (for product data and inventory)
- User Service (for user authentication and basic profile data)
- Cart Service (for cart-based recommendations)
- Order Service (for purchase history)

## Performance Requirements

### Response Time Targets
- **Personalized Recommendations**: < 200ms (95th percentile)
- **Similar Products**: < 100ms (95th percentile)
- **Trending Products**: < 50ms (95th percentile)
- **Cache Hits**: < 10ms (95th percentile)

### Throughput Targets
- **Concurrent Users**: 5,000+ simultaneous recommendation requests
- **Requests per Second**: 10,000+ RPS peak capacity
- **Daily Recommendations**: 1M+ recommendations generated per day

### Scalability Considerations
- **Horizontal Scaling**: Stateless design for easy horizontal scaling
- **Cache Efficiency**: 80%+ cache hit rate for frequently accessed data
- **Database Performance**: < 50ms for database operations
- **Memory Usage**: < 2GB RAM per service instance

This comprehensive system design provides a solid foundation for implementing the Griffin Commerce Recommendations Service, starting with simple rule-based algorithms and evolving to include machine learning capabilities while maintaining excellent observability and testability throughout.