# Caching in Go: Redis

This is an example of a simple microservice written in go. Microservice gives us the requested ID from the user database. After getting from the database, the query result is cached in Redis for 25 seconds. For logging, a zap is used, a gorilla / mux is selected as a router. For stack trace of errors use Sentry.

This example starts an http server on port 8080 at any available ip address.

> Set APP_LOG_FILE=file_name environment variable for enable output logs in project dir to file_name.

## Requirements

1. Go 1.17
2. [PostgreSQL](#postgresql): either a remote instance, local binary or docker container.
3. [Redis](#redis): either a remote instance, local binary or docker container.
4. [Load testing data](#load-testing-data): included in this repo
5. [Benchmark tests](#benchmark-tests): use a gobench tool
6. [Prometheus Metrics](#prometheus-metrics): about collected metrics
7. [Grafana Dashboards](#grafana-dashboards): recommended for use with this stack
8. [Kubernetes Probes](#kubernetes-probes): app k8s probes

## Running

> For the convenience of using all possible commands, a makefile has been prepared.

Download the required packages:

```shell script
$ make deps
```

Then you can run this example with the PostgreSQL and Redis and importing testing data:

```shell script
$ make run
```

### PostgreSQL

#### This example Using Docker Container with Postgres 12

```shell script
$ make postgres
```

### Redis

#### This example Using Docker Container with Redis 6

```shell script
$ make redis
```

### Load testing data

This repository contains test data in the sql folder to demonstrate how the example works. The data was taken from [here.](https://sample-videos.com/download-sample-sql.php)


```shell script
$ make initdb
```

### Benchmark tests

The [gobench](https://github.com/cmpxchg16/gobench) tool is used for load testing.

First install tool by command:
```shell script
$ make bench-install
```

Then you can use it with command:
```shell script
$ make stress
```

### Prometheus Metrics

For run and use ELK, Prometheus and Grafana run stack via docker-compose, before run make .env file with content:
```bazaar
DATABASE_URL=postgres://rexamp:password@postgres:5432/redisexamp?sslmode=disable
REDIS=redis:6379 
SENTRY_DSN=your_sentry_dsn
```

```shell script
$ make run-stack
```

For down this stack use:

```shell script
$ make down-stack
```

For show running containers use:

```shell script
$ make ps-stack
```

* Prometheus collect metrics from go-redis-app at port 8081 on /metrics url
* Also collect metrics from Redis and Fluent-Bit services by default settings for this services

Work with prometheus historgram on [doc](https://prometheus.io/docs/practices/histograms/) or blog [post](https://www.robustperception.io/how-does-a-prometheus-histogram-work)


### Grafana Dashboards

* [Redis Dashboard](https://grafana.com/grafana/dashboards/763)
* [Go Dashboard](https://grafana.com/grafana/dashboards/13240)
* [Fluent-Bit Dashboard](https://github.com/fluent/fluent-bit-docs/tree/8172a24d278539a1420036a9434e9f56d987a040/monitoring/dashboard.json)

### Kubernetes Probes

Application export two url on monitoring port 8081 for k8s probes.
* /live - for liveness probe, return 200 ok if live or 503 if dead
* /ready - for readiness probe, return 200 ok if live and read to working or 503 if live and not ready to working
