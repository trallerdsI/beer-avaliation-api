# Use a more recent Go version
FROM golang:1.22-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/server/main.go

# Start a new stage from scratch
FROM alpine:latest

# Install necessary packages
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/main /root/main

COPY migrations/ /root/migrations/

# Expose port 8082 to the outside world
EXPOSE 8082

# Command to run the executable
CMD ["./main"]