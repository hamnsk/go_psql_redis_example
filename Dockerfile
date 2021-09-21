FROM golang:1.17.1-alpine3.14 AS builder

WORKDIR /usr/local/go/src/

COPY ./app/ /usr/local/go/src/

RUN go clean --modcache
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o redis-cache cmd/redis-cache-example/main.go

FROM scratch
ENV DATABASE_URL=db
ENV REDIS=redis
ENV SENTRY_DSN=dsn
WORKDIR /app
COPY --from=builder /usr/local/go/src/redis-cache /app/
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/app/redis-cache"]