# Image Service Requirements

## Overview
The Image Service handles all image-related operations for the Griffin Commerce dog product store, including storage, optimization, resizing, and CDN delivery of product images.

## Package
`package images`

## Core Requirements

### Image Storage & Management
- Store original high-resolution product images
- Support multiple images per product (primary, gallery, variants)
- Organize images by product SKU and category
- Handle image uploads with validation
- Support batch image operations for bulk product updates

### Image Processing
- Automatic image optimization (WebP, AVIF formats)
- On-demand image resizing for different viewports
- Generate responsive image sets (thumbnail, small, medium, large, original)
- Smart cropping to maintain subject focus
- Lazy loading support with blur placeholders
- Image compression without quality loss

### Supported Formats
- Input: JPEG, PNG, WebP, AVIF
- Output: WebP (primary), JPEG (fallback), AVIF (modern browsers)
- Maintain original aspect ratios
- Support transparent backgrounds for PNG/WebP

### CDN Integration
- CloudFront/S3 integration for global delivery
- Automatic cache invalidation on updates
- Edge caching with appropriate TTL headers
- Bandwidth optimization through format selection
- Geographic distribution for low latency

### API Endpoints
- POST `/api/images/upload` - Upload single/multiple images
- GET `/api/images/{imageId}` - Retrieve image metadata
- GET `/api/images/{imageId}/render` - Get processed image with query params
- DELETE `/api/images/{imageId}` - Remove image
- POST `/api/images/batch` - Batch operations
- GET `/api/images/product/{productId}` - Get all images for a product

### Query Parameters for Rendering
- `width` - Desired width in pixels
- `height` - Desired height in pixels
- `format` - Output format (webp, jpeg, avif)
- `quality` - Compression quality (1-100)
- `fit` - Resize mode (cover, contain, fill, inside, outside)

### Data Models
- Image: ID, ProductID, Type (primary, gallery, variant), OriginalURL, ProcessedURLs, Metadata
- ImageMetadata: Width, Height, Format, FileSize, ColorProfile, EXIF
- ProcessedImage: ParentImageID, URL, Width, Height, Format, CacheKey
- ImageVariant: Size, Format, URL, LastModified

### Performance Requirements
- Image upload: < 5 seconds for 10MB file
- On-demand resize: < 500ms first request, < 50ms cached
- Support 10,000+ image requests per second
- 99.99% availability for image serving
- Automatic failover to backup CDN

### SEO & Accessibility
- Automatic alt text generation from product data
- Structured data for product images
- Open Graph image generation
- Support for image sitemaps
- Proper image dimensions in HTML to prevent layout shift

### Security
- Virus scanning on upload
- NSFW content detection
- File type validation
- Maximum file size limits (20MB)
- Signed URLs for temporary access
- Rate limiting on uploads

### Analytics
- Track image view counts
- Monitor bandwidth usage
- Cache hit rates
- Popular image sizes/formats
- Failed image requests

## Notes
- No backward compatibility required (greenfield project)
- Part of isolated monorepo architecture
- Use sharp/vips for image processing in Go
- Implement progressive image loading for better UX
- Consider using AI for automatic background removal for product images