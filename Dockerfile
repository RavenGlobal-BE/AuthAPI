# Stage 1: The Builder (Heavy, has all the Go tools)
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Cache dependencies first (makes future Blacksmith builds significantly faster)
COPY go.mod go.sum ./
RUN go mod download

# Copy your actual code
COPY . .

# Build a standalone, statically linked binary named 'auth'
RUN CGO_ENABLED=0 GOOS=linux go build -o auth .

# Stage 2: The Runner (Tiny, secure, no source code)
FROM alpine:latest
WORKDIR /root/

# Copy ONLY the compiled 'auth' binary from Stage 1
COPY --from=builder /app/auth .

# Expose your Nginx backend port
EXPOSE 3000

# Run the binary
CMD ["./auth"]