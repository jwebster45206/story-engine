# Multi-stage build for integration tests
FROM golang:1.22-alpine AS builder

# Install dependencies for health checks and SSL
RUN apk add --no-cache curl ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the test binary (but don't run tests yet)
RUN go build -o /dev/null ./cmd/api/...

FROM golang:1.22-alpine

RUN apk add --no-cache curl ca-certificates
WORKDIR /app
COPY --from=builder /app .

# Run tests
CMD ["go", "test", "-tags=integration", "./integration/...", "-v"]