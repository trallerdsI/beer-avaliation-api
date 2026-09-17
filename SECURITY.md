# Security Policy

## Dependabot Alerts

### GO-2026-5932: golang.org/x/crypto (openpgp)

**Status:** Accepted false positive. O projeto usa apenas `golang.org/x/crypto/bcrypt`;
nenhum código `openpgp` é importado ou executado.

## Security Hardening (Pre-Launch Audit)

### 1. Proteção de Borda

- **Rate limiting** por IP em endpoints públicos (`pkg/middleware/rate_limit.go`).
- **Payload size limit** no endpoint de busca: `maxSearchQueryLength = 200`.
- **HTTP timeouts** explícitos: `ReadTimeout(30s)`, `WriteTimeout(30s)`,
  `IdleTimeout(120s)`, `ReadHeaderTimeout(2s)` e `MaxHeaderBytes(1MB)`.
- **CORS** estrito via `CORS_ALLOWED_ORIGINS`.
- **Compressão** gzip/brotli e `Request-ID` tracing.
- `X-Forwarded-For` só é considerado quando `TRUST_PROXY=true`.

### 2. Autenticação e Autorização

- **JWT secret obrigatória:** o boot falha se `JWT_SECRET` não estiver definida.
  Testes usam `TESTING=true`.
- **Expiração:** access token com TTL de 1h e refresh token com TTL de 30 dias.
- **RBAC:** roles `user` e `admin`, com endpoints administrativos protegidos por
  `RequireAdmin`.
- **Autorização por proprietário:** operações de usuário exigem que o ID da rota
  corresponda ao usuário autenticado, salvo administradores.
- **OAuth2/OIDC:** login social Google/Apple via `id_token` RS256 + JWKS.

### 3. Banco de Dados

- **Connection pool:** `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)` e
  `SetConnMaxLifetime(5m)`.
- **Busca fuzzy:** operador `%`, índice GIN `gin_trgm_ops` e `SET LOCAL` para
  evitar vazamento de estado de sessão.
- **RLS:** políticas de acesso protegem tabelas quando o banco é acessado fora
  da API.

### 4. Event Store e Cache

- **Redis TTL + Pipeline:** eventos expiram em 72h e operações de cache são
  executadas em pipeline.
- **Persistência:** cada evento é gravado primeiro no PostgreSQL e depois no
  cache Redis.
- **Fallback PostgreSQL:** se a chave Redis expirar, o polling consulta
  `beer_events` como fonte da verdade.
- **ETag sanitizado:** weak ETags usam SHA-256 para evitar injeção de headers.

### 5. Resiliência

- **Graceful shutdown:** aguarda workers de background antes de fechar conexões.
- **Health checks:** `/healthz` é liveness; `/readyz` valida DB e Redis com
  timeout de 1s.
- **Context propagation:** operações com timeout cancelam seus contextos.

### 6. Observabilidade

- Prometheus, Grafana, alertas de erro 5xx, saturação do pool e goroutines.
- `govulncheck ./...` é executado no CI.

### 7. Testes

- `go vet ./...`
- `TESTING=true go test -race ./...`
- Testes de contrato, integração e validação dos endpoints críticos.
