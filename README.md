# Beer Review Application

A modern, scalable REST API for managing beer reviews and ratings built with Go.

## Features

- 🍺 Comprehensive beer catalog management
- 💬 User comments and ratings (persisted as JSONB embedded in the beer)
- 🔍 Advanced search with filters
- 📊 Monitoring and metrics
- 🔐 Authentication and authorization (JWT, env-configured secret)
- 🔄 Circuit breaker pattern
- 📝 Swagger documentation
- ⚡ Go 1.26 native `net/http` routing (no external router dependency)

## Architecture

The application follows clean architecture principles with the following layers:

```
beer-review-app/
├── cmd/
│   └── server/          # Application entry point
├── internal/
│   ├── app/             # Wiring (DI), DB init, migrations, router
│   ├── beer/            # Beer domain
│   │   ├── delivery/    # HTTP handlers
│   │   ├── repository/  # Data access layer
│   │   ├── usecase/     # Business logic
│   │   └── model/       # Domain models
│   ├── user/            # User domain
│   └── monitoring/      # Monitoring components
├── pkg/
│   ├── middleware/      # HTTP middleware (auth, metrics, request-id)
│   ├── auth/            # JWT helpers
│   ├── errors/          # Error handling
│   └── response/        # HTTP response helpers
```

## Tech Stack

- **Go 1.26.5** — `net/http` native routing (`log/slog`, `testing/synctest`); **zero-dependency** onde possível (UUIDv7 e Supabase Storage implementados em stdlib)
- **PostgreSQL (Supabase)** com `lib/pq`
- **Prometheus** metrics (`client_golang`)
- **golang-jwt** (HS256, stdlib `crypto/hmac`) + bcrypt para auth
- **bluemonday** HTML sanitization (XSS)
- **sony/gobreaker** circuit breaker (apenas fora de serverless)
- **Viper** configuration
- **Supabase Storage** para upload de mídia (RFC 7578) via `net/http`

## Conformidade RFC

A API segue estes RFCs (9 de 12 implementados):

| RFC | Tópico | Estado |
|-----|--------|--------|
| 9110 | HTTP Semântics | ✅ |
| 9111 | HTTP Caching (ETag/304/Cache-Control/Vary) | ✅ |
| 7519 | JWT (tokens de auth) | ✅ |
| 6750 | Bearer token no `Authorization` | ✅ |
| 9457 | Problem Details (erro único, `application/problem+json`) | ✅ |
| 8259 | JSON (UTF-8, `application/json`) | ✅ |
| 9562 | UUIDv7 (IDs de domínio, gerados na app) | ✅ |
| 7578 | Multipart/form-data (upload imagem → Supabase Storage) | ✅ |
| 6455 | SSE (tempo real; WebSocket rejeitado — ver Decisões) | ✅ |
| 8288 | Web Linking (`Link` header em listas paginadas) | ✅ |
| 6749 | OAuth2 | ⏳ pendente |
| 8030 | Web Push | ⏳ pendente |

## Decisões de Arquitetura (decisões desta fase)

- **Banco como fonte de verdade:** A API recria o esquema a cada arranque
  (`migrations/000_reset.sql` faz `DROP TABLE IF EXISTS ... CASCADE` antes de
  recriar). Migrações destrutivas são aceitáveis — não há front dependiente nem
  dados definitivos. Controlado por `DB_RESET_SCHEMA` (default `true`):
  defina `false`/`0`/`no` na Vercel quando o banco tiver dados reais para
  desativar o reset destrutivo e aplicar apenas migrations incrementais.
- **Migrações embutidas (`go:embed`):** os ficheiros SQL vivem em
  `internal/app/migrations/` e são embutidos no binário (necessário na Vercel,
  onde o filesystem do lambda não tem a pasta). O `vercel.json` builda apenas
  `api/index.go`.
- **Ligação ao Supabase na Vercel (IPv4):** o host direto `db.<ref>.supabase.co`
  só resolve para IPv6 (AAAA) e o Vercel não roteia IPv6 ("cannot assign
  requested address"). A resolução de DSN reescreve automaticamente para o
  pooler IPv4 `aws-0-<region>.pooler.supabase.com` e força
  `default_query_exec_mode=simple_protocol` (compatível com PgBouncer).
- **RLS no Supabase:** `enable_rls.sql` ativa Row Level Security em `beers`/
  `beerUsers` (leitura pública, escrita restrita a dono/admin) para fechar os
  issues do Supabase Advisor (LGPD/OWASP A05). O backend usa a service-role key
  (bypass RLS), logo a API não é afetada.
- **SSE sobre WebSocket (RFC 6455):** tempo real via Server-Sent Events
  (multiplexa sobre HTTP/2, reconexão nativa do EventSource). WebSocket só seria
  necessário para chat bidirecional privado, fora de escopo.
- **Contrato de lista vazia:** respostas paginadas serializam `[]` (nunca
  `null`) em `beers`.

## Prerequisites

- Go 1.26.5+
- PostgreSQL 17+
- Docker (optional)

## Getting Started

1. Clone the repository:
```bash
git clone https://github.com/yourusername/beer-review-app.git
cd beer-review-app
```

2. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. Run the application:
```bash
go run cmd/server/main.go
```

## API Endpoints

### Beer Operations
- `GET /api/v1/beers` - List all beers
- `POST /api/v1/beers` - Create a new beer
- `GET /api/v1/beers/{id}` - Get beer details
- `PUT /api/v1/beers/{id}` - Update beer
- `DELETE /api/v1/beers/{id}` - Delete beer
- `GET /api/v1/beers/search` - Search beers with filters

### Comments
- `POST /api/v1/beers/{id}/comments` - Add comment
- `DELETE /api/v1/beers/{id}/comments/{commentId}` - Delete comment
- `POST /api/v1/beers/{id}/comments/{commentId}/like` - Like comment

### User Operations
- `POST /api/v1/users/register` - Register new user
- `POST /api/v1/users/login` - User login
- `GET /api/v1/users/{id}` - Get user profile
- `PUT /api/v1/users/{id}` - Update profile
- `DELETE /api/v1/users/{id}` - Delete account

### Monitoring
- `GET /api/v1/health` - Health check
- `GET /api/v1/stats` - Application statistics
- `GET /metrics` - Prometheus metrics

## Testing

A suíte segue a filosofia Go "standard library first": testes table-driven com
`net/http/httptest`, mocks via interfaces implícitas e deteção de corridas.

```bash
# Formatação (o CI quebra se não estiver gofmt)
gofmt -l $(go list -f '{{.Dir}}' ./...) && git diff --exit-code

# Trindade obrigatória de CI
go vet ./...
go test -race -cover ./...

# Auditoria de cobertura
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Benchmark (serverless: alocações importam)
go test -bench=. -benchmem ./...
```

O `github/workflows/ci.yml` corre `go mod tidy`, `gofmt`, `go vet`, `go build`,
`go test -race -cover` e `govulncheck` em matrix Go `1.26.5` / `stable`.

## Deployment

### Vercel + Supabase (serverless)

A aplicação corre na Vercel como função serverless e usa PostgreSQL no Supabase.

#### 1. Banco (Supabase)
- Projeto Supabase (Free Plan ok). O esquema é recriado automaticamente a cada
  deploy via `migrateDB` (migrações embutidas em `internal/app/migrations/`).
  **Não é necessário correr SQL manualmente.**
- Recomenda-se ativar **Deployment Protection** nas definições da Vercel para
  não expor os endpoints publicamente.

#### 2. Variáveis de ambiente na Vercel
A integração Supabase injeta automaticamente `POSTGRES_URL`,
`POSTGRES_URL_NON_POOLING`, `POSTGRES_HOST`, `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DATABASE`, `SUPABASE_URL`, `SUPABASE_SERVICE_ROLE_KEY`, etc.

A resolução de DSN (`internal/app/app.go`) prioriza o **pooler IPv4** e força
`sslmode=require` + `default_query_exec_mode=simple_protocol`. Não defina
`DBConnString` apontando ao host direto (`db.<ref>.supabase.co`) — ele só
resolve para IPv6 e falha no Vercel.

Variáveis adicionais:
- `JWT_SECRET` — secreto para assinar/validar JWT (obrigatório)
- `CORS_ALLOWED_ORIGINS` — origens permitidas (ex.: `https://app.vercel.app`)
- `CORS_ALLOWED_REGEX` — regex opcional para subdomínios
- `SUPABASE_STORAGE_BUCKET` — bucket de imagens (ex.: `beer-media`)

#### 3. Deploy
O repo já inclui [vercel.json](vercel.json) e [api/index.go](api/index.go).
Ao importar na Vercel, as rotas `/api/*` são servidas pelo handler Go. O
GitHub Actions (`.github/workflows/ci.yml`) corre `gofmt`, `go vet`,
`go test -race` e `govulncheck`.

### Local (fora da Vercel)
```bash
cp .env.example .env   # ajuste DBConnString / JWT_SECRET
go run cmd/server/main.go
```

## Monitoring

The application exposes metrics for Prometheus at `/metrics` and includes:
- Response times
- Error counts
- Request counts

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.