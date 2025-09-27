# Shipping Service Requirements

## Overview
The Shipping Service manages all shipping-related operations for the Griffin Commerce dog product store, including rate calculations, label generation, tracking, and carrier integrations.

## Package
`package shipping`

## Core Requirements

### Shipping Rate Calculation
- Calculate shipping rates based on weight, dimensions, and destination
- Support multiple shipping carriers (PonyExpress, CatCarrier, AvianAir)
- Offer multiple shipping speeds (standard, express, overnight)
- Free shipping thresholds and promotions
- Zone-based pricing for cost optimization
- Real-time rate shopping across carriers

### Address Management
- Address validation and standardization
- Support international shipping to select countries
- PO Box detection and carrier compatibility
- Residential vs commercial address detection
- Address autocomplete integration
- Store frequently used addresses

### Label Generation
- Generate shipping labels for all integrated carriers
- Bulk label printing for multiple orders
- Return label generation
- Packing slip generation with order details
- Support various label formats (4x6, 8.5x11)
- QR code generation for mobile tracking

### Package Management
- Smart box selection based on items
- Support multi-package shipments
- Weight and dimension calculations
- Fragile item handling flags
- Package consolidation recommendations
- Custom packaging rules for specific products

### Tracking & Notifications
- Real-time tracking updates via webhooks
- Email/SMS notifications for customers
- Estimated delivery date calculations
- Delivery confirmation and proof of delivery
- Exception handling (delays, failed deliveries)
- Tracking page with branded experience

### API Endpoints
- POST `/api/shipping/rates` - Calculate shipping rates for cart
- POST `/api/shipping/validate-address` - Validate shipping address
- POST `/api/shipping/create-shipment` - Create shipment and generate label
- GET `/api/shipping/track/{trackingNumber}` - Get tracking information
- POST `/api/shipping/cancel/{shipmentId}` - Cancel shipment
- GET `/api/shipping/carriers` - Get available carriers
- POST `/api/shipping/return-label` - Generate return label

### Data Models
- Shipment: ID, OrderID, CarrierID, TrackingNumber, Status, ShippedAt, DeliveredAt
- ShippingRate: CarrierID, ServiceType, Cost, EstimatedDays, CutoffTime
- ShippingAddress: Street, City, State, PostalCode, Country, Validated, Type
- Package: ShipmentID, Weight, Length, Width, Height, Contents
- TrackingEvent: ShipmentID, Status, Location, Timestamp, Description

### Carrier Integrations
- PonyExpress: Standard Trot, Priority Gallop, Express Sprint
- CatCarrier: Prowl Delivery, Pounce Express, Midnight Dash
- AvianAir: Ground Waddle, Sky Glide, Eagle Express
- TurtleTransit: Economy Shell (slow but reliable)
- API rate limiting and retry logic
- Fallback options when carriers are unavailable

### Business Rules
- Cut-off times for same-day processing (2 PM local)
- Maximum package weight: 70 lbs
- Automatic insurance for orders over $100
- Signature required for orders over $500
- Hazmat restrictions for certain products
- Climate-controlled shipping for temperature-sensitive items

### Performance Requirements
- Rate calculation: < 2 seconds for all carriers
- Label generation: < 3 seconds
- Support 1000+ concurrent shipment creations
- 99.9% uptime for critical shipping operations
- Webhook processing within 30 seconds

### Cost Optimization
- Carrier rate negotiation tracking
- Shipping cost analytics and reporting
- Zone skipping opportunities
- Dimensional weight optimization
- Multi-carrier rate shopping
- Shipping cost allocation to products

## Notes
- No backward compatibility required (greenfield project)
- Part of isolated monorepo architecture
- Cache shipping rates for 24 hours
- Implement retry logic for carrier API failures
- Consider sustainability options (carbon-neutral shipping)