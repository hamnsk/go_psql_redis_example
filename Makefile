SERVICE_NAME := "redis-cache-example"
CURRENT_DIR = $(shell pwd)
GOPATH = $(shell echo ${HOME})
RELEASE ?=devel
COMMIT := git-$(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

GOOS?=linux
GOARCH?=amd64

.SILENT:

deps:
	cd ${CURRENT_DIR}/app && go mod download

clean:
	rm -rf ./.bin/${SERVICE_NAME}

clean-docker:
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

jaeger:
	docker run \
	  -d \
	  -p 6831:6831/udp \
	  -p 14268:14268 \
	  -p 16686:16686 \
	  --name rjaeger \
	  jaegertracing/all-in-one:latest
initdb:
	sleep 5
	docker exec \
	  rpsql12 \
	  psql -U rexamp -d redisexamp -f /tmp/sql/initdb.sql

build: clean deps
	cd ${CURRENT_DIR}/app && GOOS=${GOOS} GOARCH=${GOARCH} go build -mod=readonly \
	-ldflags "-s -w -X 'redis/internal/version.Version=${RELEASE}' \
	-X 'redis/internal/version.Commit=${COMMIT}' \
	-X 'redis/internal/version.BuildTime=${BUILD_TIME}'" \
	-o ./.bin/${SERVICE_NAME}${VERSION} ./cmd/${SERVICE_NAME}/main.go

dbuild: clean deps
	cd ${CURRENT_DIR}/app && GOOS=${GOOS} GOARCH=${GOARCH} go build \
	-ldflags "-s -w -X 'redis/internal/version.Version=${RELEASE}' \
	-X 'redis/internal/version.Commit=${COMMIT}' \
	-X 'redis/internal/version.BuildTime=${BUILD_TIME}'" \
	-gcflags="all=-N -l" -o ./.bin/${SERVICE_NAME}${VERSION} ./cmd/${SERVICE_NAME}/main.go \

run: clean clean-docker deps postgres redis initdb jaeger
	cd ${CURRENT_DIR}/app && \
	DATABASE_URL=postgres://rexamp:password@localhost:5432/redisexamp?sslmode=disable \
	REDIS=localhost:6379 \
	go run ${CURRENT_DIR}/app/cmd/${SERVICE_NAME}/main.go

run-stack:
	docker-compose -f docker-compose.yml up -d --build

down-stack:
	docker-compose -f docker-compose.yml down

ps-stack:
	docker-compose ps

check-live:
	curl -i http://localhost:8081/live

check-ready:
	curl -i http://localhost:8081/ready

bench-install:
	GOPATH=/tmp/ go get github.com/valyala/fasthttp
	GOPATH=/tmp/ go install github.com/cmpxchg16/gobench@latest


str:
	echo "begin stress"; \
	echo "GET http://192.168.1.43/user/1245" | vegeta attack -duration=60s -rate=100 | vegeta report & \
	wait; \
    echo "done"

stress:
	echo "begin stress"; \
	/tmp/bin/gobench -u http://192.168.1.110/user/1245 -k=true -c 100 -t 2 & \
	/tmp/bin/gobench -u http://192.168.1.110//user/4567 -k=true -c 100 -t 360 & \
	/tmp/bin/gobench -u http://192.168.1.110//user/hdfgfgh -k=true -c 100 -t 360 & \
	/tmp/bin/gobench -u http://192.168.1.110//user/647564 -k=true -c 100 -t 360 & \


stress-1:
	echo "begin stress"; \
	/tmp/bin/gobench -u http://192.168.1.108/user/1245 -k=true -c 900 -t 120 & \
	/tmp/bin/gobench -u http://192.168.1.108/user/4567 -k=true -c 900 -t 120 \
	/tmp/bin/gobench -u http://192.168.1.108/user/hdfgfgh -k=true -c 100 -t 20 & \
	/tmp/bin/gobench -u http://192.168.1.108/user/647564 -k=true -c 100 -t 20 & \
	wait; \
	echo "done"

stress-10:
	echo "begin stress"; \
	/tmp/bin/gobench -u http://192.168.1.109/user/1245 -k=true -c 100 -t 360 & \
	/tmp/bin/gobench -u http://192.168.1.109/user/4567 -k=true -c 100 -t 360 & \
	/tmp/bin/gobench -u http://192.168.1.109/user/hdfgfgh -k=true -c 100 -t 360 & \
	/tmp/bin/gobench -u http://192.168.1.109/user/647564 -k=true -c 100 -t 360 & \
	wait; \
	echo "done"

#/tmp/bin/gobench -u https://vault.k11s.cloud.vsk.local/ui -k=true -c 100 -t 360