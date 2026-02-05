# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download || go mod tidy

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o autoscaler .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates curl docker-cli docker-compose

WORKDIR /root/

COPY --from=builder /app/autoscaler .

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8090/health || exit 1

CMD ["./autoscaler"]
