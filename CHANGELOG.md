# Changelog

Todas as alterações notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/),
e este projeto adere ao [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [1.0.0] - 2026-08-21

### Added
- **Composite Event Store (`pkg/events`)**: Arquitetura Híbrida Redis (PA/EL) com fallback automático para PostgreSQL (PC/EC) em caso de expiração/miss de cache.
- **Observabilidade Completa (`pkg/metrics`, `observability/`)**:
  - Exposição de métricas do Go runtime (`GoCollector`) e processo (`ProcessCollector`).
  - Instrumentação de conexão com banco de dados (`go_sql_open_connections`, `go_sql_in_use_connections`).
  - Regras de alerta no Prometheus (`alerts.yml`) para goroutine leaks, saturação do DB pool e erros 5xx.
  - Dashboard Grafana pré-configurado (`dashboard.json`) com projeções lineares (OLS).
- **Probes Distintos (`/healthz` e `/readyz`)**:
  - `/healthz` (Liveness) com resposta incondicional.
  - `/readyz` (Readiness) com timeout de 1s validando conectividade do PostgreSQL e Redis.
- **Multi-Stage Dockerfile**: Build compilado estaticamente (`CGO_ENABLED=0`, `-ldflags="-s -w"`) utilizando imagem base Distroless e usuário não-root.
- **Pipeline de CI (`.github/workflows/ci.yml`)**:
  - Execução bloqueante de `govulncheck`.
  - Checagem de corrida de dados via `go test -race`.

### Performance & Indexação
- **Busca Difusa com FTS (`pg_trgm`)**:
  - Criação do índice `GIN` tridimensional (`add_beer_trgm_index.sql`).
  - Correção do operador de busca trigram para `%` (similaridade de Jaccard).
- **Gerenciamento de Recursos**:
  - Connection Pool do PostgreSQL explicitamente limitado (`SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5m)`).
  - Sanitização de payload de busca com trava máxima de 200 caracteres (`maxSearchQueryLength`).
  - Pré-alocação de capacidade de slices (`make([]T, 0, cap)`) para mitigar pressão de Garbage Collection no runtime.

### Security & Resiliência
- **Proteção contra DoS no HTTP Server**:
  - Configuração de `ReadHeaderTimeout` (2s), `ReadTimeout` (30s), `WriteTimeout` (30s) e `MaxHeaderBytes` (1MB).
- **Auditoria de JWT**:
  - Injeção obrigatória via variável de ambiente com `panic` na inicialização se `JWT_SECRET` for omitida.
  - Formatação sanitizada de ETag em SHA-256 (`W/"<hash>"`).
- **Graceful Shutdown**:
  - Sincronização via `sync.WaitGroup` e drenagem de conexões antes do encerramento dos pools do Redis e PostgreSQL.
