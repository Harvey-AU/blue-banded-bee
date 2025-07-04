# Build stage
FROM golang:1.25-rc-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOEXPERIMENT=greenteagc go build -o main ./cmd/app/main.go

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/main .

# Copy static files
COPY --from=builder /app/dashboard.html .
COPY --from=builder /app/web/dist ./web/dist
COPY --from=builder /app/web/examples ./web/examples

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Expose port
EXPOSE 8080

# Run the binary
CMD ["./main"]
