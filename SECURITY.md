# Security Policy

## Dependabot Alerts

### GO-2026-5932: golang.org/x/crypto (openpgp)

**Status:** Accepted false positive — no action required.

**Rationale:** The `golang.org/x/crypto` module transitively includes the
abandoned `openpgp` package, which triggers the Dependabot advisory. This
project does **not** import or execute any code from `openpgp`. The only
direct usage of `golang.org/x/crypto` is `bcrypt` for password hashing, which
is unaffected by the reported vulnerability.

**Mitigation:** The Go toolchain does not provide a way to exclude subpackages
from the module graph. Replacing `x/crypto/bcrypt` with an alternative bcrypt
library would add unnecessary risk and maintenance burden. The advisory is
acknowledged as a false positive for this codebase.

**Review date:** 2026-08-21

## Security Hardening (Pre-Launch Audit)

A suite de produção passou por auditoria rigorosa antes do launch. As medidas
implementadas incluem:

### 1. Proteção de Borda

- **Rate limiting** por IP em endpoints públicos (`middleware/ratelimit.go`).
- **Payload size limit** no endpoint de busca: `maxSearchQueryLength = 200`
  caracteres. Queries excedentes retornam `400 Bad Request` antes de alcançar
  o PostgreSQL, mitigando DoS por FTS/trgm.
- **HTTP timeouts** explícitos no servidor: `ReadTimeout(30s)`,
  `WriteTimeout(30s)`, `IdleTimeout(120s)`, `ReadHeaderTimeout(2s)`,
  `MaxHeaderBytes(1MB)` para mitigação de Slowloris.
- **CORS** estrito via `CORS_ALLOWED_ORIGINS`.
- **Compressão** gzip/brotli e `Request-ID` tracing em todas as requisições.

### 2. Autenticação e Autorização

- **JWT secret obrigatória:** O boot falha com `panic` se `JWT_SECRET` não
  estiver definida. Em testes automatizados, use `TESTING=true` para bypass.
- **Expiração de token:** TTL de 24h. O middleware valida estritamente o
  campo `exp` do JWT.
- **RBAC:** Roles `user` e `admin`. Endpoints administrativos protegidos com
  `RequireAdmin`.
- **OAuth2/OIDC:** Login social (Google/Apple) via `id_token` RS256+JWKS.

### 3. Banco de Dados

- **Connection pool sintonizado:** `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`,
  `SetConnMaxLifetime(5m)` para evitar exaustão de conexões no PostgreSQL.
- **pg_trgm correto:** Operador `%` (não `%%`) no fallback fuzzy search.
- **Índice GIN:** `gin_trgm_ops` criado em migration para busca textual.
- **SET LOCAL:** Sem vazamento de session state para `pg_trgm.similarity_threshold`.

### 4. Event Store e Cache

- **Redis TTL + Pipeline:** Eventos expiram em 72h. `ZADD`, `ZRemRangeByRank`
  e `Expire` executam em uma única Pipeline atômica.
- **Fallback PostgreSQL:** Se a chave do Redis expirar, o polling consulta
  automaticamente `beer_events` (fonte da verdade).
- **ETag sanitizado:** Weak ETags usam SHA-256 (`W/"<hash>"`) para evitar
  injeção de headers por strings brutas.

### 5. Resiliência

- **Graceful shutdown:** `sync.WaitGroup` aguarda workers de background antes
  de fechar `db.Close()` e `redis.Close()`.
- **Liveness vs Readiness:** `/healthz` retorna 200 sem tocar no banco.
  `/readyz` valida DB + Redis com timeout de 1s.
- **Context propagation:** Todos os contexts são cancelados com `defer cancel()`.

### 6. Observabilidade

- **Prometheus:** Métricas de runtime (`go_goroutines`, `process_cpu_seconds_total`)
  e aplicação (`http_requests_total`, `go_sql_*`).
- **Grafana:** Dashboard pré-configurado em `observability/grafana/dashboard.json`.
- **Alertas:** Regras em `observability/prometheus/alerts.yml` para
  vazamento de goroutines, saturação de pool e taxa de erro 5xx.
- **CI/CD:** `govulncheck ./...` como passo bloqueante no GitHub Actions.

### 7. Testes

- **Race detector:** `go test -race ./...` passa sem data races.
- **Testes de contrato:** Validam health checks, endpoints e métricas.
- **Testes de integração:** Isolados com `//go:build integration` para não
  impactar o CI rápido.
