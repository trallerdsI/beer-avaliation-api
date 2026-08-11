# Staging Environment - DoR & DoD Checklist

## 📋 DoR (Definition of Ready — Pronto para Staging)

Antes de enviar uma versão para o ambiente de Staging, verifique:

### 1. CI Verde ✅
- [ ] Pipeline de CI passou sem erros
- [ ] `go vet` sem warnings
- [ ] `go test -race` sem falhas
- [ ] Build da imagem Docker sem erros
- [ ] Scan de vulnerabilidades (Trivy/Govulncheck) sem CRITICAL/HIGH

### 2. Documentação Atualizada ✅
- [ ] `openapi.yaml` reflete alterações de payloads/endpoints
- [ ] Novos endpoints documentados com exemplos de request/response
- [ ] Códigos de erro atualizados (RFC 7807)

### 3. Migrações Validadas ✅
- [ ] Novas migrations testadas localmente
- [ ] Scripts de rollback documentados
- [ ] Dry-run executado sem erros
- [ ] Migrations são idempotentes (IF NOT EXISTS / IF EXISTS)

### 4. Segredos Configurados ✅
- [ ] Novas variáveis de ambiente adicionadas ao `.env.staging`
- [ ] Segredos cadastrados no gerenciador (GitHub Secrets, Vault, etc.)
- [ ] Credenciais de staging são DIFERENTES da produção
- [ ] Sem credenciais hardcoded no código

### 5. Mocks/Sandbox Prontos ✅
- [ ] APIs externas apontam para sandbox/mock (se aplicável)
- [ ] WireMock/Prism configurado para serviços de terceiros
- [ ] Nenhuma integração aponta para produção em staging

---

## ✅ DoD (Definition of Done — Staging Concluído)

Após o deploy no Staging, verifique:

### 1. Deploy com Sucesso ✅
- [ ] Nova imagem Docker está rodando com status Healthy
- [ ] Orquestrador reporta sucesso (Kubernetes/ECS/Docker)
- [ ] Versão implantada corresponde ao commit SHA

### 2. Migrações Aplicadas ✅
- [ ] Schema do banco reflete o estado esperado
- [ ] Nenhuma tabela bloqueada durante migração
- [ ] Dados existentes preservados (se `DB_RESET_SCHEMA=false`)

### 3. Smoke Tests Aprovados ✅
- [ ] `/api/v1/health` retorna 200 com status "healthy"
- [ ] Endpoints públicos retornam 2xx
- [ ] Fluxo de registro/login funciona
- [ ] `/docs` e `/docs/openapi.yaml` acessíveis

### 4. Observabilidade OK ✅
- [ ] Sem `panic` nos logs nas primeiras 15 minutos
- [ ] Sem picos de erro 5xx não previstos
- [ ] Métricas Prometheus sendo coletadas (se configurado)
- [ ] Logs estruturados (JSON) sendo enviados

### 5. Validação Humana ✅
- [ ] QA/Produto validou fluxos principais
- [ ] Testes E2E do app mobile/web passaram
- [ ] Performance aceitável (latência < 500ms para endpoints principais)
- [ ] Dados anonimizados (sem PII real no banco)

### 6. Segurança ✅
- [ ] Acesso restrito (VPN/IP whitelist)
- [ ] Sem dados sensíveis expostos em logs
- [ ] JWT_SECRET é diferente da produção
- [ ] CORS configurado apenas para origins de staging

---

## 🔄 Fluxo de Deploy Staging

```
┌─────────────────┐
│  Push para main │
│  ou PR mergeado │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌──────────────────┐
│   CI (Lint,     │────▶│  Build & Scan    │
│   Test, Vet)    │     │  (Trivy, Govuln) │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         ▼                       ▼
┌─────────────────┐     ┌──────────────────┐
│  Validate       │────▶│  Build & Push    │
│  Migrations     │     │  Docker Image    │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         └───────────┬───────────┘
                     ▼
           ┌─────────────────┐
           │  Deploy Staging │
           │  (Rolling/Blue- │
           │   Green)        │
           └────────┬────────┘
                    │
                    ▼
           ┌─────────────────┐
           │  Migrations     │
           │  (Auto)         │
           └────────┬────────┘
                    │
                    ▼
           ┌─────────────────┐
           │  Smoke Tests    │
           │  (Automated)    │
           └────────┬────────┘
                    │
           ┌────────┴────────┐
           │                 │
           ▼                 ▼
    ┌─────────┐      ┌──────────┐
    │  PASS   │      │  FAIL    │
    │  ✓ QA   │      │  ↺       │
    │  Test   │      │  Rollback │
    └─────────┘      └──────────┘
```

---

## 📁 Arquivos de Configuração

| Arquivo | Descrição |
|---------|-----------|
| `docker-compose.staging.yaml` | Infraestrutura isolada (PostgreSQL + API) |
| `Dockerfile.staging` | Build idêntico ao produção |
| `.env.staging.example` | Template de variáveis de ambiente |
| `scripts/anonymize_data.sql` | SQL de mascaramento de dados |
| `scripts/anonymize_data.sh` | Executor do script de anonimização |
| `scripts/smoke_tests.sh` | Testes de fumaça pós-deploy |
| `.github/workflows/staging-pipeline.yml` | Pipeline CI/CD de staging |

---

## 🚀 Comandos Úteis

### Iniciar Staging Localmente
```bash
# Copiar variáveis
cp .env.staging.example .env.staging

# Subir infraestrutura
docker compose -f docker-compose.staging.yaml up -d

# Verificar logs
docker compose -f docker-compose.staging.yaml logs -f api

# Executar anonimização (se houver dump de produção)
./scripts/anonymize_data.sh

# Rodar smoke tests
STAGING_API_URL=http://localhost:8082 ./scripts/smoke_tests.sh
```

### Parar Staging
```bash
docker compose -f docker-compose.staging.yaml down
```

### Resetar Banco de Staging
```bash
./scripts/reset_db.sh --force
```

### Verificar Anonimização
```bash
# Verificar se não há emails reais
psql $STAGING_DB_CONN -c "SELECT COUNT(*) FROM beerusers WHERE email NOT LIKE '%@staging.example.com';"

# Verificar se subscriptions foram limpas
psql $STAGING_DB_CONN -c "SELECT COUNT(*) FROM push_subscriptions;"
```

---

## ⚠️ Regras Críticas

1. **NUNCA** aponte Staging para banco de dados de Produção
2. **SEMPRE** use credenciais diferentes da produção (JWT_SECRET, OAuth, VAPID)
3. **SEMPRE** anonimize dados antes de popular staging com dump de produção
4. **NUNCA** comite arquivos `.env.staging` com credenciais reais
5. **SEMPRE** execute smoke tests após deploy antes de considerar concluído
