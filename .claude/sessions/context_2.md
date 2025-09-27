# Shopping Cart Service Requirements

## Overview
The Shopping Cart Service manages customer shopping sessions, cart operations, and cart persistence for the Griffin Commerce dog product store.

## Package
`package cart`

## Core Requirements

### Cart Management
- Create and manage unique shopping cart sessions
- Add, update, and remove items from cart
- Support guest carts and authenticated user carts
- Merge guest cart with user cart upon login
- Automatic cart expiration after configurable inactivity period (default: 30 days)

### Cart Operations
- Calculate cart totals including subtotal, taxes, and discounts
- Apply promotional codes and discounts
- Validate product availability before checkout
- Support quantity limits per product
- Handle product price changes gracefully

### Data Persistence
- Store cart data in Redis for performance
- Backup to PostgreSQL for durability
- Session-based cart for anonymous users
- User-linked cart for authenticated customers
- Cart recovery after browser/session loss

### API Endpoints
- POST `/api/cart/create` - Create new cart session
- GET `/api/cart/{cartId}` - Retrieve cart contents
- POST `/api/cart/{cartId}/items` - Add item to cart
- PUT `/api/cart/{cartId}/items/{itemId}` - Update item quantity
- DELETE `/api/cart/{cartId}/items/{itemId}` - Remove item from cart
- POST `/api/cart/{cartId}/apply-promo` - Apply promotional code
- POST `/api/cart/{cartId}/checkout` - Initialize checkout process
- POST `/api/cart/merge` - Merge guest cart with user cart

### Data Models
- Cart: ID, CustomerID (nullable), SessionID, Status, CreatedAt, UpdatedAt, ExpiresAt
- CartItem: CartID, ProductID, ProductSKU, Quantity, PriceAtAdd, CurrentPrice
- CartPromotion: CartID, PromoCode, DiscountAmount, DiscountType
- CartTotals: Subtotal, Tax, Shipping, Discount, Total

### Business Rules
- Maximum 50 unique items per cart
- Maximum quantity 99 per item
- Automatic price updates when product prices change
- Reserved inventory for 15 minutes once checkout initiated
- Cart persistence for 30 days of inactivity
- Promo codes validated in real-time

### Integration Points
- Product Catalog: Validate product availability and pricing
- Inventory Service: Check and reserve stock
- Payment Service: Pass cart totals for payment processing
- Promotions Service: Validate and apply discount codes
- Common Service: Use shared session management and caching

### Performance Requirements
- Cart operations: < 100ms response time
- Support 10,000+ concurrent active carts
- Real-time inventory checks
- Optimistic locking for concurrent cart updates

### Analytics & Tracking
- Track cart abandonment rates
- Monitor average cart value
- Log item add/remove events
- Track promotion usage
- Generate cart conversion metrics

## Notes
- No backward compatibility required (greenfield project)
- Part of isolated monorepo architecture
- Use Redis for session storage and caching
- Implement cart recovery mechanisms for better UX