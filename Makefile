.PHONY: all lint test

all: lint test

lint:
	golint ./...
	staticcheck ./...

test:
	go test ./

coverage:
	go test ./... > coverage.out
	go tool cover -html=coverage.out