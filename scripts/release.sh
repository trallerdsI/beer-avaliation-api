#!/usr/bin/env bash
set -euo pipefail

TAG="v1.0.0"
MESSAGE="Release v1.0.0 - Production Readiness & Full Observability"

echo "=== 1. Validando estado do repositório ==="
if [ -n "$(git status --porcelain)" ]; then
  echo "❌ Erro: Existem alterações não comitadas no repositório."
  exit 1
fi

echo "=== 2. Executando suíte de testes unitários e race detector ==="
TESTING=true go test -race -short ./...

echo "=== 3. Validando compilação do binário ==="
go build -o /dev/null ./cmd/server

echo "=== 4. Criando Tag Anotada ($TAG) ==="
git tag -a "$TAG" -m "$MESSAGE"

echo "=== 5. Publicando Tag no repositório remoto ==="
git push origin "$TAG"

echo "✅ Release $TAG formalizada e publicada com sucesso!"
