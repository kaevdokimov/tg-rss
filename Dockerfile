# Use the official Go image as the base image
FROM golang:1.23-alpine3.20 AS builder

ENV CGO_ENABLED=1
RUN apk add --no-cache gcc musl-dev

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the application source code
COPY . ./

# Build the Go application
RUN go build -ldflags='-s -w -extldflags "-static"' -o ./tg-rss-app

# Test Go application
RUN go test -v ./...

# Create a minimal runtime image
FROM alpine:3.20

RUN export CGO_ENABLED=1

# Set the working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/tg-rss-app /root/tg-rss-app

# Expose the application port
EXPOSE 8080

# Run the application
CMD ["/root/tg-rss-app"]

