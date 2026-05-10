#!/usr/bin/env bash
# smoke-test.sh — Self-contained stackup smoke test.
# Usage: bash smoke-test.sh ./stackup
# Requires: docker, docker compose plugin, bash 4+
# Delete this file after validation: git rm smoke-test.sh

set -euo pipefail

STACKUP="${1:-./stackup}"
PASS=0
FAIL=0
FAILED_TESTS=()
START_TIME=$(date +%s)

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
DIM='\033[2m'
RESET='\033[0m'

# ── Preflight ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${PURPLE}━━━ STACKUP SMOKE TESTS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"

if [[ ! -x "$STACKUP" ]]; then
  echo -e "${RED}ERROR: stackup binary not found or not executable: $STACKUP${RESET}"
  exit 1
fi

if ! docker compose version &>/dev/null; then
  echo -e "${RED}ERROR: docker compose not available${RESET}"
  exit 1
fi

VERSION=$("$STACKUP" version 2>&1 | head -1)
DOCKER_VER=$(docker --version | awk '{print $3}' | tr -d ',')
echo -e "  Binary:  ${DIM}$STACKUP ($VERSION)${RESET}"
echo -e "  Docker:  ${DIM}$DOCKER_VER${RESET}"
echo ""

# ── Test helpers ──────────────────────────────────────────────────────────────
TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

pass() { echo -e "  ${GREEN}PASS${RESET}  $1"; ((PASS++)); }
fail() {
  echo -e "  ${RED}FAIL${RESET}  $1"
  echo -e "       ${DIM}└─ $2${RESET}"
  ((FAIL++))
  FAILED_TESTS+=("$1")
}

# run_test <description> <expected_exit: 0|nonzero> [extra_args...]
# Runs $STACKUP in CWD with the remaining args, asserts exit code.
run_test() {
  local desc="$1" expected="$2"; shift 2
  local actual_exit=0
  "$STACKUP" "$@" </dev/null >/tmp/smoke_stdout 2>/tmp/smoke_stderr || actual_exit=$?
  if [[ "$expected" == "0" && $actual_exit -eq 0 ]]; then
    pass "$desc"
  elif [[ "$expected" == "nonzero" && $actual_exit -ne 0 ]]; then
    pass "$desc"
  elif [[ "$expected" =~ ^[0-9]+$ && $actual_exit -eq $expected ]]; then
    pass "$desc"
  else
    fail "$desc" "expected exit $expected, got $actual_exit (stdout: $(cat /tmp/smoke_stdout | head -3))"
  fi
}

compose_down() {
  docker compose down --remove-orphans -v &>/dev/null || true
}

# ── Section 1: Compose Discovery ──────────────────────────────────────────────
echo -e "${YELLOW}[ Compose Discovery ]${RESET}"

D=$(mktemp -d "$TMPROOT/disc-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && run_test "finds compose.yaml" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/disc-XXXX")
cat >"$D/compose.yml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && run_test "finds compose.yml" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/disc-XXXX")
cat >"$D/docker-compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && run_test "finds docker-compose.yaml" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/disc-XXXX")
cat >"$D/custom.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && run_test "--compose-file flag overrides discovery" 0 up -f "$D/custom.yaml")
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/disc-XXXX")
(cd "$D" && run_test "exits 1 when no compose file found" nonzero up)

echo ""

# ── Section 2: .env Gate ──────────────────────────────────────────────────────
echo -e "${YELLOW}[ .env Gate ]${RESET}"

D=$(mktemp -d "$TMPROOT/env-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
    ports:
      - "18085:80"
EOF
(cd "$D" && run_test "validate passes with no schema (nothing to validate)" 0 validate)

D=$(mktemp -d "$TMPROOT/env-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
env:
  schema:
    APP_PORT:
      required: true
EOF
cat >"$D/.env.example" <<'EOF'
APP_PORT=8080
EOF
echo "APP_PORT=8080" >"$D/.env"
(cd "$D" && run_test "validate passes with valid .env" 0 validate)

D=$(mktemp -d "$TMPROOT/env-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
env:
  schema:
    APP_PORT:
      required: true
EOF
cat >"$D/.env.example" <<'EOF'
APP_PORT=8080
EOF
echo "OTHER=value" >"$D/.env"
(cd "$D" && run_test "validate fails when required key is missing" nonzero validate)

D=$(mktemp -d "$TMPROOT/env-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
env:
  schema:
    APP_PORT:
      required: true
EOF
cat >"$D/.env.example" <<'EOF'
APP_PORT=8080
EOF
echo "APP_PORT=8080" >"$D/.env"
(cd "$D" && "$STACKUP" validate --output json </dev/null >/tmp/validate_json 2>&1) || true
if python3 -c "import json,sys; d=json.load(open('/tmp/validate_json')); sys.exit(0 if d.get('valid') else 1)" 2>/dev/null; then
  pass "validate --output json returns valid JSON with valid=true"
elif command -v python &>/dev/null && python -c "import json,sys; d=json.load(open('/tmp/validate_json')); sys.exit(0 if d.get('valid') else 1)" 2>/dev/null; then
  pass "validate --output json returns valid JSON with valid=true"
else
  fail "validate --output json returns valid JSON with valid=true" "could not verify JSON (no python available or invalid JSON)"
fi

echo ""

# ── Section 3: Health Checks ──────────────────────────────────────────────────
echo -e "${YELLOW}[ Health Checks — HTTP ]${RESET}"

D=$(mktemp -d "$TMPROOT/http-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
    ports:
      - "18081:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  web:
    health:
      type: http
      url: http://localhost:18081/
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "HTTP check passes when nginx is up" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/http-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
    ports:
      - "18081:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  web:
    health:
      type: http
      url: http://localhost:19999/
      timeout: 5s
      interval: 1s
EOF
(cd "$D" && run_test "HTTP check fails on unreachable port" nonzero up)
(cd "$D" && compose_down)

echo ""
echo -e "${YELLOW}[ Health Checks — TCP ]${RESET}"

D=$(mktemp -d "$TMPROOT/tcp-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16380:6379"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 16380
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "TCP check passes on open port" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/tcp-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16380:6379"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 19998
      timeout: 5s
      interval: 1s
EOF
(cd "$D" && run_test "TCP check fails on closed port" nonzero up)
(cd "$D" && compose_down)

echo ""
echo -e "${YELLOW}[ Health Checks — Log ]${RESET}"

D=$(mktemp -d "$TMPROOT/log-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
    ports:
      - "18082:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  web:
    health:
      type: log
      pattern: "ready for start up"
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "log check passes when pattern appears in logs" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/log-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
    ports:
      - "18082:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  web:
    health:
      type: log
      pattern: "PATTERN_THAT_NEVER_APPEARS_xyzzy_99999"
      timeout: 8s
      interval: 1s
EOF
(cd "$D" && run_test "log check times out when pattern never appears" nonzero up)
(cd "$D" && compose_down)

echo ""

# ── Section 4: Startup Sequencing ─────────────────────────────────────────────
echo -e "${YELLOW}[ Startup Sequencing ]${RESET}"

D=$(mktemp -d "$TMPROOT/tier-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: test
    ports:
      - "15432:5432"
  cache:
    image: redis:7-alpine
    ports:
      - "16381:6379"
  web:
    image: nginx:alpine
    ports:
      - "18083:80"
    depends_on:
      - db
      - cache
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  db:
    health:
      type: tcp
      host: localhost
      port: 15432
      timeout: 60s
      interval: 2s
  cache:
    health:
      type: tcp
      host: localhost
      port: 16381
      timeout: 30s
      interval: 1s
  web:
    health:
      type: http
      url: http://localhost:18083/
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "multi-tier: all 3 services start healthy" 0 up)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/only-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
  web:
    image: nginx:alpine
    ports:
      - "18080:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 16379
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "--only cache starts only cache" 0 up --only cache)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/profile-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16382:6379"
  web:
    image: nginx:alpine
    ports:
      - "18084:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
profiles:
  backend:
    services:
      - cache
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 16382
      timeout: 30s
      interval: 1s
EOF
(cd "$D" && run_test "--profile backend starts only cache" 0 up --profile backend)
(cd "$D" && compose_down)

D=$(mktemp -d "$TMPROOT/partial-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16380:6379"
  ghost:
    image: nginx:alpine
    ports:
      - "18086:80"
EOF
cat >"$D/stackup.yml" <<'EOF'
version: "1"
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 16380
      timeout: 30s
      interval: 1s
  ghost:
    health:
      type: tcp
      host: localhost
      port: 19997
      timeout: 5s
      interval: 1s
EOF
(cd "$D" && run_test "--partial returns exit code 3 on partial success" 3 up --partial)
(cd "$D" && compose_down)

echo ""

# ── Section 5: CLI Commands ───────────────────────────────────────────────────
echo -e "${YELLOW}[ CLI Commands ]${RESET}"

D=$(mktemp -d "$TMPROOT/cmds-XXXX")
(cd "$D" && run_test "stackup version prints version" 0 version)

D=$(mktemp -d "$TMPROOT/cmds-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
EOF
(cd "$D" && run_test "stackup init generates stackup.yml" 0 init)
[[ -f "$D/stackup.yml" ]] && pass "stackup.yml was created" || fail "stackup.yml was created" "file not found"

D=$(mktemp -d "$TMPROOT/cmds-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  web:
    image: nginx:alpine
EOF
echo "existing" >"$D/stackup.yml"
(cd "$D" && run_test "stackup init refuses to overwrite existing stackup.yml" nonzero init)

D=$(mktemp -d "$TMPROOT/cmds-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && run_test "stackup doctor runs without panic" 0 doctor)

D=$(mktemp -d "$TMPROOT/cmds-XXXX")
cat >"$D/compose.yaml" <<'EOF'
services:
  cache:
    image: redis:7-alpine
    ports:
      - "16379:6379"
EOF
(cd "$D" && "$STACKUP" up </dev/null >/dev/null 2>&1 || true)
(cd "$D" && run_test "stackup down stops all containers" 0 down)

echo ""

# ── Results ────────────────────────────────────────────────────────────────────
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
TOTAL=$((PASS + FAIL))

echo -e "${PURPLE}━━━ RESULTS ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "  ${GREEN}Passed:${RESET}   $PASS/$TOTAL"
if [[ $FAIL -gt 0 ]]; then
  echo -e "  ${RED}Failed:${RESET}   $FAIL/$TOTAL"
  for t in "${FAILED_TESTS[@]}"; do
    echo -e "  ${DIM}  └─ $t${RESET}"
  done
fi
echo -e "  ${DIM}Duration: ${DURATION}s${RESET}"
echo ""

[[ $FAIL -eq 0 ]]
