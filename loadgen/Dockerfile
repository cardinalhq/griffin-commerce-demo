FROM python:3.11-slim

# Set working directory
WORKDIR /loadgen

# Install dependencies
RUN pip install --no-cache-dir locust==2.17.0

# Copy the locust files
COPY locustfile.py .
COPY config.py .

# Expose Locust web UI port
EXPOSE 8089

# Set environment variables for memory efficiency
ENV PYTHONUNBUFFERED=1
ENV LOCUST_LOGLEVEL=INFO

# Default command - can be overridden
CMD ["locust", "-f", "locustfile.py"]