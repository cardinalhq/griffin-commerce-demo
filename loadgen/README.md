# Load Generator for Griffin Commerce Demo

This directory contains a Locust-based load generator that simulates realistic user behavior for the Griffin Commerce Demo e-commerce platform.

## Features

- **Realistic User Simulation**: Tests through the frontend (port 5173) just like real users
- **Multiple User Personas**:
  - Regular users (60%) - Normal browsing and shopping behavior
  - Mobile users (30%) - Slower interactions, more browsing, less checkout
  - Power users (10%) - Faster interactions, more purchases
- **Full User Journey**: Browse → View Products → Load Images → Add to Cart → Checkout
- **Memory Optimized**: Runs efficiently in Docker with configurable memory limits

## Quick Start

### Local Development

1. Install Locust:

```bash
pip install locust
```

1. Run with Web UI:

```bash
cd loadgen
locust -f locustfile.py --host http://localhost:5173
```

1. Open browser to <http://localhost:8089>

### Docker Compose

The load generator is included in docker-compose.yml with memory limits:

```bash
# Run entire stack including load generator
docker-compose up

# Run load generator only
docker-compose up loadgen

# Adjust load dynamically via environment variables
LOCUST_USERS=100 LOCUST_SPAWN_RATE=5 docker-compose up loadgen
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOCUST_USERS` | 10 | Number of concurrent users |
| `LOCUST_SPAWN_RATE` | 1 | Users spawned per second |
| `LOCUST_RUN_TIME` | 60s | Test duration (e.g., 60s, 5m, 1h) |
| `NORMAL_USER_PCT` | 60 | Percentage of normal users |
| `MOBILE_USER_PCT` | 30 | Percentage of mobile users |
| `POWER_USER_PCT` | 10 | Percentage of power users |
| `FRONTEND_HOST` | <http://localhost:5173> | Frontend URL for local testing |
| `FRONTEND_PROD_HOST` | <http://frontend:5173> | Frontend URL in Docker |

### Memory Limits

The Docker container is configured with:

- Maximum memory: 256MB
- Reserved memory: 128MB

These limits keep resource usage low while supporting hundreds of simulated users.

## User Scenarios

### Browse Catalog (30% weight)

- GET `/` (homepage)
- GET `/api/products`
- Simulates browsing with realistic delays

### View Product Details (25% weight)

- GET `/api/products/{id}`
- GET `/static/products/{id}.jpg` (product image)
- GET `/api/recommendations/product/{id}`

### Add to Cart (20% weight)

- POST `/api/cart/{id}/add`
- Adds 1-3 items randomly

### View Cart (15% weight)

- GET `/api/cart/{id}`

### Checkout (3% weight)

- POST `/api/cart/{id}/checkout`
- Creates new cart after successful checkout

## Monitoring

### Locust Web UI (port 8089)

- Real-time request statistics
- Response time charts
- Request/failure rates
- Download CSV reports

### Key Metrics

- **Response Time Percentiles**: p50, p95, p99
- **Requests Per Second**: Overall throughput
- **Failure Rate**: Should stay under 1%
- **Active Users**: Current concurrent users

## Running Load Tests

### Interactive Mode (with Web UI)

```bash
# Start Locust with web interface
docker-compose up loadgen
# Open http://localhost:8089
# Configure users and spawn rate
# Start swarming
```

### Headless Mode (CI/CD friendly)

```bash
# Run 100 users for 5 minutes
docker run --rm \
  --network griffin-network \
  -e LOCUST_USERS=100 \
  -e LOCUST_SPAWN_RATE=10 \
  -e LOCUST_RUN_TIME=5m \
  griffin-loadgen \
  locust -f /loadgen/locustfile.py \
    --host http://frontend:5173 \
    --headless
```

### Dynamic Load Adjustment

You can adjust load in real-time through the Locust web UI:

1. Navigate to <http://localhost:8089>
2. Use the "Edit" button to change user count
3. Watch metrics update in real-time

## Performance Tuning

### For Higher Loads

1. Increase spawn rate gradually:

```bash
LOCUST_SPAWN_RATE=20 docker-compose up loadgen
```

1. Run distributed Locust:

```bash
# Master
locust -f locustfile.py --master --host http://localhost:5173

# Workers (run multiple)
locust -f locustfile.py --worker --master-host=localhost
```

1. Adjust memory limits in docker-compose.yml if needed:

```yaml
deploy:
  resources:
    limits:
      memory: 512M
```

## Troubleshooting

### Connection Errors

- Ensure all services are running: `docker-compose ps`
- Check frontend is accessible: `curl http://localhost:5173`
- Verify network connectivity in Docker

### Slow Response Times

- Start with fewer users and gradually increase
- Check backend service logs for errors
- Monitor CPU/memory on host machine

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
- name: Run Load Test
  run: |
    docker-compose up -d
    sleep 30  # Wait for services
    docker-compose run --rm loadgen \
      locust -f /loadgen/locustfile.py \
      --host http://frontend:5173 \
      --headless \
      --users 50 \
      --spawn-rate 5 \
      --run-time 2m \
      --html report.html
```
