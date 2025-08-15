FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for any dependencies that might need it
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy config files to the image
COPY config/ ./config/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mcp-server ./cmd/mcp-server

FROM alpine:latest

# Install ca-certificates for HTTPS requests and git for operations
RUN apk --no-cache add ca-certificates git curl

WORKDIR /app

# Create directories for projects and config
RUN mkdir -p /app/projects /app/config

# Copy the binary from builder
COPY --from=builder /app/mcp-server .

# Copy config files from builder
COPY --from=builder /app/config/ ./config/

# Expose the MCP server port
EXPOSE 8080

# Set default environment variables
ENV PORT=8080
ENV OLLAMA_URL=http://ollama:11434
ENV PROJECT_ROOT=/app/projects

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the server
CMD ["./mcp-server"]