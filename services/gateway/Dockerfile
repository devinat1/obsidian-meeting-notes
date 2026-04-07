# services/gateway/Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY shared/ shared/
COPY gateway/ gateway/

RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway-service ./gateway/cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /gateway-service /gateway-service

EXPOSE 8080

ENTRYPOINT ["/gateway-service"]
