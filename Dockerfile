FROM golang:1.17.1-alpine3.14 AS builder

WORKDIR /usr/local/go/src/

COPY ./app/ /usr/local/go/src/

RUN go clean --modcache
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -o redis-cache cmd/redis-cache-example/main.go

FROM scratch
ENV DATABASE_URL=postgres://rexamp:password@postgres:5432/redisexamp?sslmode=disable
ENV REDIS=redis:6379
WORKDIR /app
COPY --from=builder /usr/local/go/src/redis-cache /app/

CMD ["/app/redis-cache"]