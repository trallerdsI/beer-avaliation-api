.PHONY: all lint test

all: lint test

lint:
	go vet ./...
	staticcheck ./...

test:
	go test ./

coverage:
	go test ./... > coverage.out
	go tool cover -html=coverage.out