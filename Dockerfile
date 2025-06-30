# Build stage
FROM golang:1.24.1-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o laq ./cmd/laq

# Final stage
FROM alpine:3.21

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S lacquer && \
    adduser -u 1000 -S lacquer -G lacquer

# Copy binary from builder
COPY --from=builder /build/laq /usr/local/bin/laq

# Set ownership and permissions
RUN chmod +x /usr/local/bin/laq

# Switch to non-root user
USER lacquer

# Set working directory
WORKDIR /workspace

# Expose port (if needed for serve command)
EXPOSE 8080

# Default command
ENTRYPOINT ["laq"]
CMD ["--help"]