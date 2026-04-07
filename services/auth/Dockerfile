FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY shared/ shared/
COPY auth/ auth/

RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./auth/cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /auth-service /auth-service
COPY auth/static/ /static/

EXPOSE 8081

ENTRYPOINT ["/auth-service"]
