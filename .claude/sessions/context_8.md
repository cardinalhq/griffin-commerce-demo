# Furry-Themed Service Names Reference

## Payment Processors

### PuppyPay
- Primary payment gateway
- Supports all major card types
- Fast authorization and capture
- Good reliability

### KittyCard
- Alternative payment processor
- Known for being unpredictable (20% failure rate in testing)
- Slightly higher latency
- Good for testing failure scenarios

### DoggieCoin
- Cryptocurrency payment option
- Very fast processing (50ms latency)
- Blockchain-based settlements
- Popular with tech-savvy customers

### PawPal
- Digital wallet service
- One-click payments
- Stored payment methods
- Social payment features

## Shipping Carriers

### PonyExpress
- Traditional ground shipping
- Service levels:
  - Standard Trot (5-7 business days)
  - Priority Gallop (2-3 business days)
  - Express Sprint (1 business day)
- Reliable with 95% success rate

### CatCarrier
- Premium shipping option
- Service levels:
  - Prowl Delivery (3-5 business days)
  - Pounce Express (1-2 business days)
  - Midnight Dash (overnight)
- Unpredictable (10% timeout rate) but fast when working

### AvianAir
- Air freight specialist
- Service levels:
  - Ground Waddle (economy ground, 7-10 days)
  - Sky Glide (standard air, 2-3 days)
  - Eagle Express (priority overnight)
- Best for lightweight packages

### TurtleTransit
- Budget shipping option
- Service level:
  - Economy Shell (10-14 business days)
- Slow but extremely reliable
- Lowest cost option

## Other Services

### HootMail
- Email service provider
- Transactional and marketing emails
- Owl-themed templates
- Night-time delivery optimization

### ChirpText
- SMS notification service
- Order updates and tracking
- Bird-song notification sounds
- Character-efficient messaging

### FurryFast CDN
- Content delivery network
- Image optimization and caching
- Global paw-print (presence)
- Supports all modern image formats

### NestFinder
- Address validation service
- Geocoding and standardization
- Delivery point validation
- Bird's-eye view mapping

## Testing Scenarios

### Payment Testing
- **Happy Path**: Use PuppyPay or DoggieCoin (0% failure rate)
- **Failure Testing**: Use KittyCard (20% failure rate)
- **Latency Testing**: Use PawPal (150ms latency)
- **Quick Transactions**: Use DoggieCoin (50ms latency)

### Shipping Testing
- **Reliable Delivery**: Use PonyExpress or TurtleTransit
- **Fast Delivery**: Use CatCarrier (when it works) or AvianAir
- **Failure Scenarios**: Use CatCarrier (10% timeout rate)
- **Budget Testing**: Use TurtleTransit

## Fault Injection Examples

```yaml
# Make KittyCard fail 50% of the time
fault_injection:
  payment:
    kittycard:
      failure_rate: 0.5
      failure_type: "declined"

# Make CatCarrier always timeout
fault_injection:
  shipping:
    catcarrier:
      failure_rate: 1.0
      failure_type: "timeout"

# Add latency to PonyExpress
fault_injection:
  shipping:
    ponyexpress:
      failure_rate: 0.0
      latency_ms: 3000  # 3 second delay
```

## Brand Guidelines

### Naming Conventions
- Payment services use domestic animal themes (dogs, cats)
- Shipping services use various animal locomotion themes
- Communication services use animal sounds/calls
- All names should be playful and memorable

### Service Personalities
- **PuppyPay**: Friendly, reliable, eager to please
- **KittyCard**: Independent, sometimes moody, premium feel
- **DoggieCoin**: Modern, tech-forward, loyal community
- **PonyExpress**: Traditional, steady, dependable
- **CatCarrier**: Fast but unpredictable, luxurious
- **AvianAir**: Swift, elevated service, bird's eye efficiency
- **TurtleTransit**: Slow and steady, budget-conscious

## Implementation Notes

- All services are mocked in the POC
- Failure rates and latencies are configurable via YAML
- Each service has distinct characteristics for realistic testing
- Service behaviors can be modified in real-time for demos
- OpenTelemetry tracks all interactions with these services