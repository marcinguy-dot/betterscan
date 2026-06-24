FROM golang:1.22 AS builder

WORKDIR /app

# 1. Copy the module files first
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy EVERYTHING else (including the models and worker folders)
COPY . .

# 3. Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/main .

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./main"]
