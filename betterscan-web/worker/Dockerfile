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

# git: clone repos. bash/curl/wget/tar: betterscan tool installers (opengrep, trivy, …).
# python3/py3-pip: bandit installer. openjdk optional for cpg/joern (heavy; install on demand).
RUN apk --no-cache add ca-certificates git bash curl wget tar gzip \
	gcc musl-dev python3 py3-pip

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/worker .

# Create temp directory for scans
RUN mkdir -p /tmp/betterscan-scans

# Expose port (if needed for health checks)
EXPOSE 8081

# Run the binary
CMD ["./worker"]
