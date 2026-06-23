# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and other dependencies needed for scanning
RUN apk add --no-cache git gcc musl-dev


# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o worker .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git gcc musl-dev

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/worker .

# Create temp directory for scans
RUN mkdir -p /tmp/checkmate-scans

# Expose port (if needed for health checks)
EXPOSE 8081

# Run the binary
CMD ["./worker"]
