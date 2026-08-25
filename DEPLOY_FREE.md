# Deploy 100% Gratuito (Sem Cartão)

Stack: **Render** (API) + **Neon.tech** (Postgres) + **Upstash** (Redis)

## 1. PostgreSQL — Neon.tech

1. Crie conta em https://neon.tech
2. Crie um projeto (ex: `beer-api-prod`)
3. No SQL Editor, habilite a extensão:
   ```sql
   CREATE EXTENSION IF NOT EXISTS pg_trgm;
   ```
4. Copie a `DATABASE_URL` do projeto (formato `postgres://...`)

## 2. Redis — Upstash

1. Crie conta em https://upstash.com
2. Crie um banco Redis na região mais próxima
3. Copie a `REDIS_URL` (formato `rediss://...`)

## 3. API Go — Render

1. Crie conta em https://render.com
2. **New +** → **Web Service**
3. Conecte o repositório `trallerdsI/beer-avaliation-api` ou selecione **Deploy from Docker image**
4. Se for imagem Docker:
   - Registry: `GitHub`
   - Repository: `trallerdsI/beer-avaliation-api`
   - Tag: `v1.0.1`
5. Configure as variáveis de ambiente:
   ```
   DATABASE_URL=<DATABASE_URL do Neon>
   REDIS_URL=<REDIS_URL do Upstash>
   JWT_SECRET=<openssl rand -hex 32>
   ENV=production
   CORS_ALLOWED_ORIGINS=https://seu-dominio.com,https://app.seu-dominio.com
   PORT=8080
   ```
6. Clique em **Create Web Service**

## 4. Validação Pós-Deploy

```bash
# Health check
curl -i https://beer-avaliation-api.onrender.com/api/v1/health

# Readiness
curl -i https://beer-avaliation-api.onrender.com/readyz

# Métricas
curl -i https://beer-avaliation-api.onrender.com/metrics
```

## Observações

- O plano free do Render entra em hibernação após 15 min sem tráfego (cold start ~30s)
- O Neon pode pausar após 7 dias sem atividade (basta acessar o painel para reativar)
- Para evitar o cold start, configure um cron job externo que faça ping em `/api/v1/health` a cada 10 minutos
