#!/usr/bin/env bash
# smoke_tests.sh — Testes de fumaça pós-deploy para Staging.
#
# Uso:
#   ./scripts/smoke_tests.sh [BASE_URL]
#
# Exemplo:
#   ./scripts/smoke_tests.sh https://staging-api.example.com
#
# Este script valida que os endpoints principais respondem corretamente
# após um deploy. Retorna 0 se todos os testes passarem, 1 caso contrário.

set -euo pipefail

BASE_URL="${1:-http://localhost:8082}"
REPORT_FILE="smoke-test-results-$(date +%Y%m%d-%H%M%S).json"

PASSED=0
FAILED=0
RESULTS="[]"

add_result() {
  local name="$1"
  local status="$2"
  local http_code="$3"
  local details="$4"

  RESULTS=$(echo "$RESULTS" | python3 -c "
import sys, json
results = json.load(sys.stdin)
results.append({
    'name': '$name',
    'status': '$status',
    'http_code': $http_code,
    'details': '$details',
    'timestamp': __import__('datetime').datetime.now().isoformat()
})
print(json.dumps(results, indent=2))
")

  if [ "$status" = "PASS" ]; then
    PASSED=$((PASSED + 1))
  else
    FAILED=$((FAILED + 1))
  fi
}

run_test() {
  local name="$1"
  local method="$2"
  local endpoint="$3"
  local expected_code="$4"
  local body="$5"

  echo "Testing: $name"

  if [ "$method" = "POST" ]; then
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
      -H "Content-Type: application/json" \
      -d "$body" \
      "$BASE_URL$endpoint" 2>/dev/null) || true
  else
    RESPONSE=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$endpoint" 2>/dev/null) || true
  fi

  HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
  BODY=$(echo "$RESPONSE" | head -n-1)

  if [ "$HTTP_CODE" = "$expected_code" ]; then
    echo "  ✓ PASS (HTTP $HTTP_CODE)"
    add_result "$name" "PASS" "$HTTP_CODE" "OK"
  else
    echo "  ✗ FAIL (expected $expected_code, got $HTTP_CODE)"
    echo "  Response: $BODY"
    add_result "$name" "FAIL" "$HTTP_CODE" "Expected $expected_code, got $HTTP_CODE"
  fi
}

echo "========================================"
echo "  SMOKE TESTS - STAGING"
echo "  URL: $BASE_URL"
echo "========================================"
echo ""

# 1. Health Check
run_test "Health Check" "GET" "/api/v1/health" "200" ""

# 2. Public Stats
run_test "Public Stats" "GET" "/api/v1/stats" "200" ""

# 3. Beer Enums
run_test "Beer Enums" "GET" "/api/v1/beers/enums" "200" ""

# 4. List Beers
run_test "List Beers" "GET" "/api/v1/beers" "200" ""

# 5. Search Beers
run_test "Search Beers" "GET" "/api/v1/beers/search?query=ipa" "200" ""

# 6. Register User (test with staging data)
run_test "Register User" "POST" "/api/v1/users/register" "201" '{
  "username": "staging_test_user",
  "email": "staging_test@example.com",
  "password": "staging_test_123"
}'

# 7. Login
run_test "Login" "POST" "/api/v1/users/login" "200" '{
  "email": "staging_test@example.com",
  "password": "staging_test_123"
}'

# 8. OpenAPI Docs
run_test "OpenAPI Docs" "GET" "/docs" "200" ""

# 9. OpenAPI Spec
run_test "OpenAPI Spec" "GET" "/docs/openapi.yaml" "200" ""

# Summary
echo ""
echo "========================================"
echo "  RESULTS"
echo "========================================"
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo ""

python3 -c "
import json
results = json.loads('$RESULTS')
report = {
    'timestamp': __import__('datetime').datetime.now().isoformat(),
    'base_url': '$BASE_URL',
    'total': len(results),
    'passed': $PASSED,
    'failed': $FAILED,
    'results': results
}
with open('$REPORT_FILE', 'w') as f:
    json.dump(report, f, indent=2)
print(f'Report saved to: $REPORT_FILE')
"

if [ "$FAILED" -gt 0 ]; then
  echo "ERRO: $FAILED teste(s) falharam!"
  exit 1
fi

echo "Todos os smoke tests passaram!"
exit 0
