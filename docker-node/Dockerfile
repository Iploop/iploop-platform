FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o iploop-node .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/iploop-node /usr/local/bin/iploop-node
ENTRYPOINT ["iploop-node"]
