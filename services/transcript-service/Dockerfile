FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /transcript-service ./cmd/transcript

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /transcript-service /transcript-service
EXPOSE 8082
ENTRYPOINT ["/transcript-service"]
