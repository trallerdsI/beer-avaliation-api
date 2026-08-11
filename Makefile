.PHONY: all lint fmt test ci reset-db staging-up staging-down staging-logs staging-smoke staging-anonymize

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

# ---------------------------------------------------------------------------
# STAGING
# ---------------------------------------------------------------------------

staging-up:
	@if [ ! -f .env.staging ]; then \
		echo "ERRO: .env.staging não encontrado. Copie .env.staging.example primeiro."; \
		exit 1; \
	fi
	docker compose -f docker-compose.staging.yaml up -d
	@echo "Aguardando PostgreSQL ficar saudável..."
	@docker compose -f docker-compose.staging.yaml exec -T postgres pg_isready -U $$(grep STAGING_DB_USER .env.staging | cut -d= -f2 || echo postgres)

staging-down:
	docker compose -f docker-compose.staging.yaml down

staging-logs:
	docker compose -f docker-compose.staging.yaml logs -f api

staging-logs-db:
	docker compose -f docker-compose.staging.yaml logs -f postgres

staging-smoke:
	@URL=$$(grep STAGING_API_URL .env.staging 2>/dev/null | cut -d= -f2 || echo http://localhost:8082); \
	./scripts/smoke_tests.sh $$URL

staging-anonymize:
	./scripts/anonymize_data.sh

staging-reset:
	@echo "ATENÇÃO: Isto irá apagar todos os dados do banco de staging!"
	@read -p "Confirmar? [yes/N] " ans; \
	if [ "$$ans" = "yes" ]; then \
		CONN=$$(grep STAGING_DB_CONN_STRING .env.staging 2>/dev/null | cut -d= -f2); \
		if [ -n "$$CONN" ]; then \
			psql "$$CONN" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"; \
			echo "Schema resetado."; \
		else \
			echo "STAGING_DB_CONN_STRING não configurado em .env.staging"; \
			exit 1; \
		fi \
	else \
		echo "Abortado."; \
	fi
