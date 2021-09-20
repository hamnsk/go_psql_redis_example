SERVICE_NAME := "redis-cache-example"
CURRENT_DIR = $(shell pwd)
ifdef BUILD_VERSION
	VERSION := "-$(BUILD_VERSION)"
else
	VERSION := ""
endif

.SILENT:

deps:
	go mod download

clean:
	rm -rf ./.bin/${SERVICE_NAME}

clean_docker:
	docker stop rpsql12 rredis6 || true && docker rm rpsql12 rredis6 || true

postgres:
	docker run \
      -d \
      -e POSTGRES_HOST_AUTH_METHOD=trust \
      -e POSTGRES_USER=rexamp \
      -e POSTGRES_PASSWORD=password \
      -e POSTGRES_DB=redisexamp \
      -p 5432:5432 \
      -v ${CURRENT_DIR}/sql:/tmp/sql \
      --name rpsql12 \
      postgres:12.5-alpine

redis:
	docker run \
      -d \
      -p 6379:6379 \
      --name rredis6 \
      redis:6.2.1-alpine3.13

initdb:
	sleep 5
	docker exec \
	  rpsql12 \
	  psql -U rexamp -d redisexamp -f /tmp/sql/initdb.sql

build: clean deps
	CGO_ENABLED=0 GOOS=linux go build -o ./.bin/${SERVICE_NAME}${VERSION} ./app/cmd/${SERVICE_NAME}/main.go

run: clean clean_docker deps postgres redis initdb
	DATABASE_URL=postgres://rexamp:password@localhost:5432/redisexamp?sslmode=disable \
	REDIS=localhost:6379 \
	go run ${CURRENT_DIR}/app/cmd/${SERVICE_NAME}/main.go

bench-install:
	GOPATH=/tmp/ go get github.com/valyala/fasthttp
	GOPATH=/tmp/ go get github.com/cmpxchg16/gobench

stress:
	/tmp/bin/gobench -u http://localhost:8080/user/1 -k=true -c 500 -t 10