FROM golang:latest AS builder

WORKDIR /app

# 1. Copy the module files first
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy EVERYTHING else (including the models and worker folders)
COPY . .

# 3. Build the application
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
