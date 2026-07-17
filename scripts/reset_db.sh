#!/usr/bin/env bash
# reset_db.sh — Limpa todas as tabelas da aplicação na base apontada por
# DBConnString / DB_CONN_STRING (lê de .env se presente).
#
# ⚠️  CUIDADO: isto elimina TODOS os dados (cervejas, comentários, utilizadores).
# Apenas para bases de teste/desenvolvimento. NUNCA corras contra produção.
#
# Uso:
#   ./scripts/reset_db.sh            # pede confirmação interativa
#   ./scripts/reset_db.sh --force    # sem confirmação (CI / uso consciente)
set -euo pipefail

# Carrega .env se existir (sem sobrescrever vars de ambiente já definidas).
if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
fi

CONN="${DB_CONN_STRING:-${DBConnString:-}}"
if [ -z "$CONN" ]; then
  echo "ERRO: DBConnString/DB_CONN_STRING não definido." >&2
  exit 1
fi

if [ "${1:-}" != "--force" ]; then
  read -r -p "Isto vai APAGAR todas as tabelas (beers, beerusers, comments). Continuar? [yes/N] " ans
  if [ "${ans:-N}" != "yes" ]; then
    echo "Abortado."
    exit 0
  fi
fi

PSQL="psql ${CONN}"
echo "A remover tabelas..."
${PSQL} -c "DROP TABLE IF EXISTS comments, beers, beerusers CASCADE;"
echo "Concluído. As migrations recriam o schema no próximo arranque da API."
