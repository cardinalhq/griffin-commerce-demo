# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Griffin Commerce Demo is a microservices-based e-commerce platform with a Go backend and Svelte frontend. The platform consists of six microservices (catalog, cart, payment, shipping, images, recommendations) that communicate via REST APIs.

## Development Commands

### Backend Services

Build the Griffin CLI (single binary for all services):

```bash
make                    # or make build
# OR directly:
go build -o bin/griffin .
```

Run services via CLI subcommands:

```bash
./bin/griffin catalog
./bin/griffin cart
./bin/griffin payment
./bin/griffin shipping
./bin/griffin images
./bin/griffin recommendations
```

### Frontend

```bash
cd frontend
npm install      # Install dependencies
npm run dev      # Start development server (port 5173)
npm run build    # Build for production
npm run check    # Run Svelte checks and TypeScript validation
```

### Testing & Quality

```bash
make test              # Run all unit tests
make integration-test  # Run integration tests
make fmt               # Format all Go code
make lint              # Run golangci-lint
make check             # Run fmt, test, and lint
```

### Docker & Deployment

```bash
docker-compose up      # Run entire stack
make docker-build      # Build Docker images locally
make images           # Build and push multi-arch images
```

## Architecture

### Service Communication Pattern

All backend services follow a consistent pattern:

1. Services import from `common` package for shared functionality (middleware, telemetry, models)
2. Each service has a `Start()` function that initializes telemetry, sets up routing with Gorilla Mux, and starts HTTP server
3. Services use environment variables for configuration (PORT, SERVICE_NAME)
4. Default service ports: catalog(8080), payment(8081), cart(8082), shipping(8083), images(8084), recommendations(8085)

### Key Architectural Components

**common/** - Shared package containing:

- Middleware (logging, CORS, tracing, correlation ID)
- Telemetry initialization (OpenTelemetry integration)
- Database utilities
- Error handling
- Shared models and configurations

**services/** - Individual microservices that handle specific domains:

- Each service is self-contained with its own main.go, routes, and handlers
- Services are stateless and designed to run in containers
- Inter-service communication happens via HTTP REST APIs

**frontend/** - Svelte 5 application using:

- Vite for bundling and development
- TypeScript for type safety
- Tailwind CSS for styling

### Service Startup Pattern

When modifying or rebuilding services, follow this pattern:

```bash
# Kill existing service if running
pkill -f service-name

# Build service
cd services/service-name
go build -o ../../bin/service-name .

# Run service
cd ../..
./bin/service-name
```

### Configuration

Services read configuration from:

1. Environment variables (PORT, SERVICE_NAME, etc.)
2. config.yaml for shared settings
3. products.yaml for catalog service product data

The entrypoint.sh script handles Docker container initialization, determining which service to run based on SERVICE_NAME environment variable.
