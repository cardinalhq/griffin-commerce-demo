# Griffin Commerce Frontend

A simple, clean e-commerce frontend for pet products built with Svelte and TailwindCSS.

## Tech Stack

- **Framework**: Svelte 5 + Vite
- **Styling**: TailwindCSS
- **Language**: TypeScript
- **Icons**: Built-in emojis (no icon library needed for simplicity)

## Quick Start

```bash
# Install dependencies
npm install

# Run development server
npm run dev
# Open http://localhost:5173

# Build for production
npm run build

# Preview production build
npm run preview
```

## Features

- ✅ Product listing from backend API
- ✅ Responsive grid layout
- ✅ Stock indicators
- ✅ Loading states
- ✅ Error handling
- 🚧 Shopping cart functionality
- 🚧 Product detail pages
- 🚧 Checkout flow

## Backend Services

Make sure the backend services are running:

```bash
# From the root directory
make all  # Builds all services

# Run each service in separate terminals:
./bin/catalog-service    # Port 8080
./bin/cart-service       # Port 8082
./bin/payment-service    # Port 8081
./bin/shipping-service   # Port 8083
./bin/images-service     # Port 8084
./bin/recommendations-service # Port 8085
```

## Project Structure

```
frontend/
├── src/
│   ├── App.svelte       # Main application component
│   ├── app.css          # TailwindCSS imports
│   └── main.ts          # Application entry point
├── public/              # Static assets
└── package.json
```

## Development

The app uses TailwindCSS for styling with a simple color scheme:
- Primary: Purple (`bg-purple-600`)
- Background: Gray (`bg-gray-50`)
- Surface: White
- Text: Gray shades

## CORS Configuration

The backend services are configured to allow CORS from the frontend development server (http://localhost:5173).

## Next Steps

To extend the application:

1. **Add Cart Functionality**: Create a cart store using Svelte stores
2. **Product Details**: Add routing with a product detail page
3. **Checkout Flow**: Implement multi-step checkout
4. **User Authentication**: Add login/signup
5. **Image Support**: Integrate with the image service

## Notes

This is a simplified e-commerce demo focusing on:
- Clean, functional design
- Fast performance
- Easy to understand code
- Minimal dependencies