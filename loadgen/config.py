"""
Configuration for Locust load testing
"""
import os

# Target host configuration - Frontend server that proxies to backend services
FRONTEND_HOST = os.environ.get("FRONTEND_HOST", "http://localhost:5173")
FRONTEND_PROD_HOST = os.environ.get("FRONTEND_PROD_HOST", "http://frontend:3000")

# Load test parameters
USERS = int(os.environ.get("LOCUST_USERS", "10"))
SPAWN_RATE = int(os.environ.get("LOCUST_SPAWN_RATE", "1"))
RUN_TIME = os.environ.get("LOCUST_RUN_TIME", "60s")

# User distribution (percentages)
USER_CLASSES = {
    "EcommerceUser": int(os.environ.get("NORMAL_USER_PCT", "60")),
    "MobileUser": int(os.environ.get("MOBILE_USER_PCT", "30")),
    "PowerUser": int(os.environ.get("POWER_USER_PCT", "10"))
}

# Performance thresholds for monitoring
THRESHOLDS = {
    "response_time_95": 1000,  # 95th percentile should be under 1s
    "response_time_99": 2000,  # 99th percentile should be under 2s
    "failure_rate": 0.01,      # Less than 1% failure rate
}

# Headers to simulate real browsers
BROWSER_HEADERS = {
    "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
    "Accept": "text/html,application/json,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
    "Accept-Language": "en-US,en;q=0.5",
    "Accept-Encoding": "gzip, deflate",
    "Connection": "keep-alive",
}

# Memory optimization settings
MAX_RESPONSE_SIZE = int(os.environ.get("MAX_RESPONSE_SIZE", "10485760"))  # 10MB max response to store
RESET_STATS_INTERVAL = int(os.environ.get("RESET_STATS_INTERVAL", "3600"))  # Reset stats every hour to save memory