# ---- Build stage ----
FROM golang:1.24-alpine3.22 AS builder

# Install build dependencies for CGO (needed by modernc.org/sqlite)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go.mod and go.sum first (better caching for Docker layers)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code (including configs)
COPY . .

# Build the Go binary with CGO enabled and strip debug symbols
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o server ./cmd/main.go



# ---- Runtime stage ----
FROM alpine:3.22

# Install runtime dependencies (needed by CGO binary)
RUN apk --no-cache add ca-certificates libstdc++

WORKDIR /app


COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./server"]
