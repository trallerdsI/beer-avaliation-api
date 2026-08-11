#!/usr/bin/env bash
# anonymize_data.sh — Script de anonimização de dados para Staging.
#
# Uso:
#   ./scripts/anonymize_data.sh [--dry-run] [--full]
#
# Opções:
#   --dry-run   Mostra o que seria executado sem modificar dados
#   --full      Executa anonimização completa (inclui reset de estatísticas)
#
# Este script deve ser executado APENAS em banco de dados de Staging.
# NUNCA execute contra Produção ou Desenvolvimento.
#
# Pré-requisitos:
#   - psql instalado
#   - Variável DB_CONN_STRING ou DBConnString configurada apontando para o
#     banco de dados de Staging (NUNCA para Produção)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DRY_RUN=false
FULL=false

for arg in "$@"; do
  case $arg in
    --dry-run) DRY_RUN=true ;;
    --full) FULL=true ;;
    *)
      echo "Opção desconhecida: $arg" >&2
      echo "Uso: $0 [--dry-run] [--full]" >&2
      exit 1
      ;;
  esac
done

CONN="${DB_CONN_STRING:-${DBConnString:-}}"
if [ -z "$CONN" ]; then
  echo "ERRO: DB_CONN_STRING/DBConnString não definida." >&2
  echo "Configure a variável apontando para o banco de STAGING." >&2
  exit 1
fi

if [[ "$CONN" == *"production"* ]] || [[ "$CONN" == *"prod"* ]]; then
  echo "ERRO DE SEGURANÇA: Esta conexão parece apontar para Produção!" >&2
  echo "Este script só deve ser executado contra o banco de Staging." >&2
  exit 1
fi

SQL_FILE="${SCRIPT_DIR}/anonymize_data.sql"

if [ ! -f "$SQL_FILE" ]; then
  echo "ERRO: Arquivo de SQL não encontrado: $SQL_FILE" >&2
  exit 1
fi

echo "========================================"
echo "  ANONIMIZAÇÃO DE DADOS - STAGING"
echo "========================================"
echo ""
echo "Banco alvo: $(echo "$CONN" | sed 's/:\/\/[^:]*:[^@]*@/:\/\/***:***@/')"
echo "Modo: $([ "$DRY_RUN" = true ] && echo 'DRY-RUN (apenas validação)' || echo 'EXECUÇÃO')"
echo ""

if [ "$DRY_RUN" = true ]; then
  echo "--- Conteúdo do SQL que seria executado ---"
  cat "$SQL_FILE"
  echo ""
  echo "--- Fim do dry-run ---"
  echo "Execute sem --dry-run para aplicar as alterações."
  exit 0
fi

echo "Executando anonimização..."
if [ "$FULL" = true ]; then
  echo "Modo FULL: incluindo reset de estatísticas."
fi

set +e
psql "$CONN" -v ON_ERROR_STOP=1 -f "$SQL_FILE" 2>&1
EXIT_CODE=$?
set -e

if [ $EXIT_CODE -eq 0 ]; then
  echo ""
  echo "✓ Anonimização concluída com sucesso."
  echo ""
  echo "Verificações pós-anonimização:"
  echo "  1. SELECT COUNT(*) FROM beerusers WHERE email NOT LIKE '%@staging.example.com';"
  echo "     (deve retornar 0)"
  echo "  2. SELECT COUNT(*) FROM push_subscriptions;"
  echo "     (deve retornar 0)"
else
  echo ""
  echo "✗ Erro durante a anonimização (código: $EXIT_CODE)"
  echo "Verifique os logs acima para detalhes."
  exit $EXIT_CODE
fi
