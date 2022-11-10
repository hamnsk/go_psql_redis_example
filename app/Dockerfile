FROM golang:1.18-alpine3.14 AS builder
COPY . /usr/local/go/src/redis-cache
WORKDIR /usr/local/go/src/redis-cache

RUN go clean --modcache
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o redis-cache cmd/redis-cache-example/main.go

FROM scratch
ENV DATABASE_URL=db
ENV REDIS=redis
ENV SENTRY_DSN=dsn
WORKDIR /app
COPY --from=builder /usr/local/go/src/redis-cache/redis-cache /app/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/app/redis-cache"]