# Caching in Go: Redis

This is an example of a simple microservice written in go. Microservice gives us the requested ID from the user database. After getting from the database, the query result is cached in Redis for 25 seconds. For logging, a zap is used, a gorilla / mux is selected as a router.

This example starts an http server on port 8080 at any available ip address.

> Set APP_LOG_FILE=file_name environment variable for enable output logs in project dir to file_name.

## Requirements

1. Go 1.17
2. [PostgreSQL](#postgresql): either a remote instance, local binary or docker container.
3. [Redis](#redis): either a remote instance, local binary or docker container.
4. [Load testing data](#load-testing-data): included in this repo
5. [Benchmark tests](#benchmark-tests): use a gobench tool

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