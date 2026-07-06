FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bin/proxy ./cmd/proxy

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/bin/proxy ./bin/proxy

EXPOSE 3000

CMD ["./bin/proxy", "-config", "/app/config.json"]
