.PHONY: all lint test coverage

all: lint test

lint:
	go vet ./...

test:
	go test ./... -coverprofile=coverage.out

coverage:
	go tool cover -html=coverage.out
