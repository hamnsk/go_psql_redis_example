FROM golang:1.18-alpine3.14 AS certs

FROM scratch
ENV DATABASE_URL=db
ENV REDIS=redis
ENV SENTRY_DSN=dsn
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
WORKDIR /app
COPY redis-cache /app/redis-cache

CMD ["/app/redis-cache"]