# Build stage
FROM golang:1.27.1-alpine AS builder

WORKDIR /app

# Copy dependency files first
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build Go application
RUN go build -o billing-api ./cmd/api

# Runtime stage
FROM alpine:3.22

WORKDIR /app

# Copy compiled binary
COPY --from=builder /app/billing-api .

# Expose application port
EXPOSE 8080

# Start application
CMD ["./billing-api"]