.PHONY: build run test

PROJECT?=payment-gateway
LINTER_VERSION=v1.33.0

default: build

build: test build-local

build-local:
	go build -o ./app ./cmd/server

run: build
	./app

run-local: build-local
	./app

build-docker: test
	docker-compose build

run-docker: build-docker
	docker-compose up

stop-docker:
	docker-compose stop

clean-docker:
	docker-compose rm -s -f

lint: get-linter
	golangci-lint run --timeout=5m

get-linter:
	command -v golangci-lint || curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b ${GOPATH}/bin $(LINTER_VERSION)

test: generate lint
	go fmt ./...
	go test -vet all ./...

test-race:
	go test -v -race ./...

test-integration:
	go test -v -vet all -tags=integration ./... -coverprofile=integration.out

test-all: test test-race test-integration

cover:
	echo unit tests only
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

generate: get-generator
	go generate -x ./...

get-generator:
	go install github.com/golang/mock/mockgen@v1.6.0

clean-mock:
	find internal -iname '*_mock.go' -exec rm {} \;

regenerate: clean-mock generate

mod:
	go mod vendor -v

docs:
	godoc -http=:6060
