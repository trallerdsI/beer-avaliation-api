# Deploy no Fly.io

## Pré-requisitos

- Conta no [Fly.io](https://fly.io)
- [Fly CLI](https://fly.io/docs/hands-on/install-flyctl/) instalado
- Autenticado: `fly auth login`

## Variáveis de ambiente

A aplicação requer as seguintes variáveis em produção:

| Variável | Descrição | Obrigatória |
|----------|-----------|-------------|
| `JWT_SECRET` | Chave secreta para assinatura de JWT | Sim |
| `DB_CONN_STRING` | Connection string do PostgreSQL | Sim |
| `REDIS_URL` | URL do Redis (opcional, fallback para PG) | Não |
| `CORS_ALLOWED_ORIGINS` | Origens permitidas (CSV) | Sim |
| `ENV` | Ambiente (`production`) | Sim |

## Provisionamento

```bash
# 1. Criar app (sem deploy imediato)
fly launch --no-deploy --name beer-avaliation-api --region gru

# 2. Criar Postgres gerenciado
fly postgres create --name beer-db --region gru --initial-cluster-size 1 --volume-size 10

# 3. Attach do banco na app
fly postgres attach beer-db

# 4. Criar Redis gerenciado
fly redis create --name beer-redis --region gru

# 5. Configurar secrets
fly secrets set \
  JWT_SECRET="$(openssl rand -hex 32)" \
  DB_CONN_STRING="postgres://..." \
  REDIS_URL="redis://..." \
  CORS_ALLOWED_ORIGINS="https://seu-dominio.com" \
  ENV="production"
```

## Extensão pg_trgm

```bash
fly postgres connect -a beer-db
# No psql:
CREATE EXTENSION IF NOT EXISTS pg_trgm;
\q
```

## Deploy

```bash
fly deploy
```

## Validação

```bash
# Health check
curl https://beer-avaliation-api.fly.dev/api/v1/health

# Métricas
curl https://beer-avaliation-api.fly.dev/metrics

# Logs
fly logs
```

## Escala

```bash
# Escalar máquinas
fly scale vm shared-cpu-1x --memory 512

# Escalar instâncias
fly scale count 2
```

## Notas

- A imagem é publicada automaticamente no GHCR via tag `v*`
- O Fly.io usa a porta `8080` internamente (configurado em `fly.toml`)
- SSL/HTTPS é gerenciado automaticamente pelo Fly.io
- Para rollback: `fly deploy --image ghcr.io/trallerdsI/beer-avaliation-api:v1.0.0`
