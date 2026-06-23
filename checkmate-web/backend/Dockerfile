FROM golang:1.22 AS builder

WORKDIR /app

# 1. Copy the module files first
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy EVERYTHING else (including the models and worker folders)
COPY . .

# 3. Build pointing to the SPECIFIC directory of the worker, NOT the root
RUN CGO_ENABLED=0 GOOS=linux go build -o backend ./backend

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
