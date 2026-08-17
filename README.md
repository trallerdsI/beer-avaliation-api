# Beer Review Application

A modern, scalable REST API for managing beer reviews and ratings built with Go.

## Features

- 🍺 Comprehensive beer catalog management
- 💬 User comments and ratings (persisted as JSONB embedded in the beer)
- 🔍 Advanced search with filters and 409 similarity prevention
- 📊 Admin analytics (`/admin/stats`) and user personal stats (`/users/me/stats`)
- 🔐 Authentication and authorization (JWT HS256 + OAuth2/OIDC RS256)
- 📝 Swagger documentation
- ⚡ Go 1.26 native `net/http` routing (no external router dependency)
- 🛡️ Rate limiting (429), CORS, gzip/brotli compression, request ID tracing
- 🔔 Push subscriptions (Web Push / Push API)
- 📤 Media upload via multipart/form-data (RFC 7578) to Supabase Storage

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
│   │   ├── delivery/
│   │   ├── repository/
│   │   ├── usecase/
│   │   └── model/
│   └── monitoring/      # Health + stats controllers
├── pkg/
│   ├── middleware/      # HTTP middleware (auth, metrics, request-id, CORS, compression, rate-limit)
│   ├── auth/            # JWT helpers (HS256 session + RS256 OIDC validation)
│   ├── errors/          # RFC 7807 Problem Details
│   ├── response/        # HTTP response helpers
│   ├── metrics/         # Prometheus metrics + serverless detection
│   ├── storage/         # Supabase Storage upload adapter (RFC 7578)
│   ├── uuid/            # UUIDv7 generator (stdlib, RFC 9562)
│   ├── validation/      # Custom validators (flavor, aroma, color, etc.)
│   └── vercel/          # IPv4 DSN resolution for Supabase pooler
```

## Tech Stack

- **Go 1.26.5** — `net/http` native routing (`log/slog`); **zero-dependency** where possible (UUIDv7 stdlib)
- **PostgreSQL 17+ (Supabase)** com `lib/pq`
- **golang-jwt/v5** — HS256 session tokens + RS256 OIDC validation (exceção à regra Zero-Dependency)
- **go-playground/validator/v10** — validação de domínio
- **bluemonday** — sanitização HTML (XSS)
- **Prometheus client_golang** — métricas
- **testify** — mocks e assertions em testes

## Conformidade RFC

A API segue estes RFCs (12 de 12 implementados):

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
| 6749 | OAuth2 / OIDC (login social Google/Apple via id_token RS256+JWKS) | ✅ |
| 8030 | Web Push | ✅ |

## Modelo de Dados

- **`beers`** — Catálogo de cervejas com `style`, `taste`, `aroma`, `color`, `body`, `carbonation`, `finish`
- **`comments`** — JSONB embutido em `beers` (modelo documento, não tabela separada)
- **`beerUsers`** — Utilizadores com `role` (`user`/`admin`), `provider` (local/google/apple), `external_sub`
- **`push_subscriptions`** — Subscrições Web Push por utilizador

## Decisões de Arquitetura

- **Banco como fonte de verdade:** `DB_RESET_SCHEMA=true` (default) recria o esquema a cada arranque via `internal/app/migrations/000_reset.sql`. Defina `false`/`0`/`no` na Vercel para preservar dados e aplicar apenas migrations incrementais.
- **Login social (RFC 6749 / OIDC):** `POST /api/v1/users/oauth` recebe `provider` + `id_token` (JWT RS256 do Google/Apple). Valida contra JWKS do IdP com cache e faz upsert em `beerUsers` por `(provider, external_sub)`. Devolve JWT HS256 de sessão.
- **Migrações embutidas (`go:embed`):** os ficheiros SQL vivem em `internal/app/migrations/` e são embutidos no binário (necessário na Vercel, onde o filesystem do lambda não tem a pasta).
- **Ligação ao Supabase na Vercel (IPv4):** o host direto `db.<ref>.supabase.co` só resolve para IPv6. A resolução de DSN reescreve automaticamente para o pooler IPv4 `aws-0-<region>.pooler.supabase.com` e força `default_query_exec_mode=simple_protocol`.
- **RLS no Supabase:** `enable_rls.sql` ativa Row Level Security em `beers`/`beerUsers`. O backend usa service-role key (bypass RLS).
- **SSE sobre WebSocket (RFC 6455):** tempo real via Server-Sent Events (multiplexa sobre HTTP/2). WebSocket só para chat bidirecional privado, fora de escopo.
- **Rate Limiter:** cleanup lazy de chaves expiradas no map `hits` para evitar OOM em serverless.
- **Contrato de erro RFC 7807:** `code` estável (snake_case) + `detail` + `instance` (caminho da rota). O Flutter mapeia `code` para `DSLanguageError` / `DSLanguageFeedback`.

## API Endpoints

### Beer Operations

- `GET /api/v1/beers` - List all beers
- `POST /api/v1/beers` - Create a new beer
- `GET /api/v1/beers/{id}` - Get beer details
- `PUT /api/v1/beers/{id}` - Update beer
- `DELETE /api/v1/beers/{id}` - Delete beer
- `GET /api/v1/beers/search` - Search beers with filters
- `POST /api/v1/beers/{id}/comments` - Add comment
- `DELETE /api/v1/beers/{id}/comments/{commentId}` - Delete comment
- `POST /api/v1/beers/{id}/comments/{commentId}/like` - Like comment
- `POST /api/v1/beers/{id}/media` - Upload media (RFC 7578)

### User Operations

- `POST /api/v1/users/register` - Register new user
- `POST /api/v1/users/login` - User login
- `POST /api/v1/users/oauth` - OAuth2/OIDC login (Google/Apple)
- `GET /api/v1/users/{id}` - Get user profile
- `PUT /api/v1/users/{id}` - Update profile
- `DELETE /api/v1/users/{id}` - Delete account
- `POST /api/v1/users/{id}/push/subscribe` - Subscribe to push
- `POST /api/v1/users/{id}/push/unsubscribe` - Unsubscribe from push
- `GET /api/v1/users/{id}/push` - List push subscriptions

### Monitoring

- `GET /api/v1/health` - Health check
- `GET /api/v1/stats` - Public statistics (total beers + top styles)
- `GET /api/v1/admin/stats` - Admin statistics panel (requires admin role)
- `GET /api/v1/users/me/stats` - Authenticated user personal statistics
- `GET /metrics` - Prometheus metrics (non-serverless only)

## Testing

```bash
# Formatação (o CI quebra se não estiver gofmt)
gofmt -l $(go list -f '{{.Dir}}' ./...) && git diff --exit-code

# Trindade obrigatória de CI
go vet ./...
go test -race -cover ./...

# Auditoria de vulnerabilidades
govulncheck ./...

# Benchmark (serverless: alocações importam)
go test -bench=. -benchmem ./...
```

## Deployment

### Vercel + Supabase (serverless)

A aplicação corre na Vercel como função serverless e usa PostgreSQL no Supabase.

#### 1. Banco (Supabase)

- Projeto Supabase (Free Plan ok). O esquema é recriado automaticamente a cada
  deploy via `migrateDB` (migrações embutidas em `internal/app/migrations/`).
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
- `DB_RESET_SCHEMA` — `true`/`false` para controlar reset do schema no arranque
- `CORS_ALLOWED_ORIGINS` — origens permitidas (ex.: `https://app.vercel.app`)
- `CORS_ALLOWED_REGEX` — regex opcional para subdomínios
- `SUPABASE_STORAGE_BUCKET` — bucket de imagens (ex.: `beer-media`)
- `OAUTH_GOOGLE_AUDIENCE` — client ID da app Flutter no Google. Defina para ativar login Google.
- `OAUTH_GOOGLE_JWKS_URL` — **opcional**; default `https://www.googleapis.com/oauth2/v3/certs`.
- `OAUTH_APPLE_AUDIENCE` — Service ID da app no Apple Developer. Defina para ativar login Apple.
- `OAUTH_APPLE_JWKS_URL` — **opcional**; default `https://appleid.apple.com/auth/keys`.

#### 3. Deploy

O repo já inclui [vercel.json](vercel.json) e [api/index.go](api/index.go).
Ao importar na Vercel, as rotas `/api/*` são servidas pelo handler Go.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
