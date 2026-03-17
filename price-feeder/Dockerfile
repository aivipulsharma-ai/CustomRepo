# Build stage
FROM golang:1.25.3-alpine AS builder

# Set working directory
WORKDIR /build

# Install git (needed for go modules)
RUN apk add --no-cache git

# Copy core module first (dependency from ../../core)
COPY core/ ./core/

# Copy price-feeder module
COPY dextr-avs/price-feeder/ ./dextr-avs/price-feeder/

# Set working directory to price-feeder
WORKDIR /build/dextr-avs/price-feeder

# Download dependencies (replace directive will resolve ../../core to /build/core)
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /build/dextr-avs/price-feeder/main .

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose ports
# 8080 for the main HTTP server
# 9091 for Prometheus metrics (configurable via config)
EXPOSE 8080 9091

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
