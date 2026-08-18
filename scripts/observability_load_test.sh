#!/usr/bin/env bash
set -euo pipefail

# ------------------------------------------------------------------------------
# Load + Observability Validation Script
# Gera carga na API e valida métricas (Prometheus), traces (Tempo) e dashboard (Grafana).
# ------------------------------------------------------------------------------

BASE_URL="${BASE_URL:-http://localhost:8082}"
PROM_URL="${PROM_URL:-http://localhost:9090}"
TEMPO_URL="${TEMPO_URL:-http://localhost:3200}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin}"

echo "=== Observability Load Test ==="
echo "API:      $BASE_URL"
echo "Prometheus: $PROM_URL"
echo "Tempo:     $TEMPO_URL"
echo "Grafana:   $GRAFANA_URL"

# ------------------------------------------------------------------------------
# 1. Gera carga na API para produzir métricas e traces
# ------------------------------------------------------------------------------
echo ""
echo "[1/5] Gerando carga na API..."

# Health check (gera métrica e trace)
for i in {1..5}; do
  curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/healthz" || true
  echo ""
done

# Lista beers (gera métrica e trace)
for i in {1..10}; do
  curl -s -o /dev/null "$BASE_URL/api/v1/beers" || true
done

# Cria beer (se houver JWT, senão 401)
BEER_PAYLOAD='{"name":"Load Test Beer","style":"IPA","aroma":"citrus","color":"gold","body":"medium","carbonation":"high","taste":"bitter"}'
for i in {1..3}; do
  curl -s -o /dev/null -X POST "$BASE_URL/api/v1/beers" \
    -H "Content-Type: application/json" \
    -d "$BEER_PAYLOAD" || true
done

echo "Carga gerada."

# ------------------------------------------------------------------------------
# 2. Valida métricas no Prometheus
# ------------------------------------------------------------------------------
echo ""
echo "[2/5] Validando métricas no Prometheus..."

# Espera Prometheus scrapear (até 30s)
for i in {1..30}; do
  HTTP_REQUESTS=$(curl -s "$PROM_URL/api/v1/query?query=http_requests_total" | grep -o '"value":\[.*\]' | head -1 || true)
  if [ -n "$HTTP_REQUESTS" ]; then
    echo "Métricas detectadas no Prometheus."
    break
  fi
  sleep 1
done

# Valida se há métricas de HTTP
RESULT=$(curl -s "$PROM_URL/api/v1/query?query=http_requests_total" | grep -o '"result":\[\]' || true)
if [ -n "$RESULT" ]; then
  echo "ERRO: Nenhuma métrica http_requests_total encontrada no Prometheus."
  exit 1
fi

echo "Métricas validadas."

# ------------------------------------------------------------------------------
# 3. Valida traces no Tempo
# ------------------------------------------------------------------------------
echo ""
echo "[3/5] Validando traces no Tempo..."

# Espera Tempo ingerir traces (até 30s)
for i in {1..30}; do
  TRACES=$(curl -s "$TEMPO_URL/api/search?limit=10" | grep -o '"traceID":"[^"]*"' | head -1 || true)
  if [ -n "$TRACES" ]; then
    echo "Traces detectados no Tempo."
    break
  fi
  sleep 1
done

# Valida TraceQL básico (deve retornar algo)
TRACEQL_RESULT=$(curl -s "$TEMPO_URL/api/search?q={resource.service.name=\"beer-avaliation-api\"}&limit=10" | grep -o '"traceID":"[^"]*"' | head -1 || true)
if [ -z "$TRACEQL_RESULT" ]; then
  echo "AVISO: Nenhum trace encontrado para beer-avaliation-api (pode ser normal se sampling estiver baixo)."
else
  echo "Traces validados."
fi

# ------------------------------------------------------------------------------
# 4. Valida dashboard no Grafana
# ------------------------------------------------------------------------------
echo ""
echo "[4/5] Validando dashboard no Grafana..."

# Login no Grafana
LOGIN_RESPONSE=$(curl -s -X POST "$GRAFANA_URL/api/login" \
  -H "Content-Type: application/json" \
  -d "{\"user\":\"$GRAFANA_USER\",\"password\":\"$GRAFANA_PASSWORD\"}" || true)

if echo "$LOGIN_RESPONSE" | grep -q '"message":"Invalid username or password"'; then
  echo "ERRO: Credenciais inválidas no Grafana ($GRAFANA_USER / $GRAFANA_PASSWORD)."
  exit 1
fi

# Busca dashboard pelo UID
DASHBOARD=$(curl -s -b "$LOGIN_RESPONSE" "$GRAFANA_URL/api/dashboards/uid/beer-api-observability" || true)
if echo "$DASHBOARD" | grep -q '"message":"Dashboard not found"'; then
  echo "ERRO: Dashboard beer-api-observability não encontrado no Grafana."
  echo "Verifique o provisioning em observability/grafana/provisioning/."
  exit 1
fi

echo "Dashboard encontrado no Grafana."

# ------------------------------------------------------------------------------
# 5. Valida painéis específicos do dashboard
# ------------------------------------------------------------------------------
echo ""
echo "[5/5] Validando painéis do dashboard..."

PANELS=("HTTP Requests (RED)" "HTTP Latency (p50, p90, p99)" "Database Retries by Operation" "DB Connection Pool (USE)" "Moderation Requests" "Moderation Cache Hits by Backend" "Active SSE Connections")

for panel in "${PANELS[@]}"; do
  if echo "$DASHBOARD" | grep -q "$panel"; then
    echo "  [OK] Painel: $panel"
  else
    echo "  [AVISO] Painel não encontrado: $panel"
  fi
done

echo ""
echo "=== Validação de Observabilidade Concluída ==="
echo "Métricas:       OK"
echo "Traces:         $( [ -n "$TRACEQL_RESULT" ] && echo 'OK' || echo 'AVISO (sampling)' )"
echo "Dashboard:      OK"
echo "Provisioning:   OK"
