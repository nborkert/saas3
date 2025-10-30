# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/compliancesync-api ./cmd/api

# Final stage
FROM gcr.io/distroless/static-debian11:nonroot

# Copy the binary from builder
COPY --from=builder /app/compliancesync-api /compliancesync-api

# Expose port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/compliancesync-api"]
