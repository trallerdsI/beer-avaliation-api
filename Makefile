.PHONY: all lint fmt test ci reset-db

all: lint fmt test

# Análise estática nativa (Pilar 1 — Zero-Dependency).
lint:
	go vet ./...

# Formatação obrigatória (igual ao CI) para evitar drift de estilo.
fmt:
	gofmt -w $(shell go list -f '{{.Dir}}' ./...)

# Testes com -race (Pilar 3: concorrência sem race conditions) + cobertura.
test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Alias para o pipeline de CI local (build + vet + fmt + race tests).
ci:
	go build ./...
	go vet ./...
	unformatted=$$(gofmt -l $$(go list -f '{{.Dir}}' ./...)); \
	if [ -n "$$unformatted" ]; then echo "Ficheiros não formatados:"; echo "$$unformatted"; exit 1; fi
	go test -race ./...

# Gap C: limpa todas as tabelas da base apontada por DBConnString.
# ⚠️  Apenas para bases de teste — NUNCA correr contra produção.
reset-db:
	./scripts/reset_db.sh

coverage:
	go tool cover -html=coverage.out
