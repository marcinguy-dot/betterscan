# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o betterscan .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates git

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/betterscan .

# Install common security scanning tools
RUN apk --no-cache add \
    trivy \
    bandit \
    && rm -rf /var/cache/apk/*

# Create directory for rules
RUN mkdir -p /root/.betterscan/rules

ENTRYPOINT ["./betterscan"]
