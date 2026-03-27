#!/bin/bash
# E2E test for the airlock proxy decryption pipeline.
# Requires: Docker running, airlock-claude:latest and airlock-proxy:latest built,
#           airlock binary at bin/airlock.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AIRLOCK_BIN="${AIRLOCK_BIN:-$PROJECT_ROOT/bin/airlock}"
SESSION_ID="e2e-proxy-$$"
WORKDIR=""
PASS=0
FAIL=0
TESTS=()

# Auto-detect Docker socket if DOCKER_HOST is not set or docker is unreachable
if [ -z "${DOCKER_HOST:-}" ] || ! docker info &>/dev/null; then
    for sock in "$HOME/.rd/docker.sock" "$HOME/.colima/docker.sock" "/var/run/docker.sock"; do
        if [ -S "$sock" ]; then
            export DOCKER_HOST="unix://$sock"
            break
        fi
    done
fi

# --- Helpers ---

cleanup() {
    if [ -n "$WORKDIR" ]; then
        "$AIRLOCK_BIN" stop --id "$SESSION_ID" 2>/dev/null || true
        rm -rf "$WORKDIR"
    fi
}
trap cleanup EXIT

log_pass() {
    PASS=$((PASS + 1))
    TESTS+=("PASS: $1")
    printf "  \033[32mPASS\033[0m %s\n" "$1"
}

log_fail() {
    FAIL=$((FAIL + 1))
    TESTS+=("FAIL: $1 -- $2")
    printf "  \033[31mFAIL\033[0m %s -- %s\n" "$1" "$2"
}

container_exec() {
    docker exec "airlock-claude-$SESSION_ID" airlock-exec.sh bash -c "$1"
}

# --- Setup ---

setup() {
    printf "\n=== Setup ===\n"

    # Verify prerequisites
    if ! command -v docker &>/dev/null; then
        echo "ERROR: docker not found" >&2; exit 1
    fi
    if ! docker info &>/dev/null; then
        echo "ERROR: Docker daemon not running" >&2; exit 1
    fi
    if [ ! -x "$AIRLOCK_BIN" ]; then
        echo "ERROR: airlock binary not found at $AIRLOCK_BIN (run 'make build')" >&2; exit 1
    fi
    if ! docker image inspect airlock-claude:latest &>/dev/null; then
        echo "ERROR: airlock-claude:latest image not found (run 'make docker-build')" >&2; exit 1
    fi
    if ! docker image inspect airlock-proxy:latest &>/dev/null; then
        echo "ERROR: airlock-proxy:latest image not found (run 'make docker-build')" >&2; exit 1
    fi

    WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/airlock-e2e-XXXXXX")
    cd "$WORKDIR"

    "$AIRLOCK_BIN" init >/dev/null 2>&1

    cat > .env << 'EOF'
TEST_SECRET=hello_from_airlock
STRIPE_KEY=sk_live_test_12345
QUOTED_DOUBLE="double_quoted_value"
QUOTED_SINGLE='single_quoted_value'
EOF

    printf "Starting session %s...\n" "$SESSION_ID"
    local output
    output=$("$AIRLOCK_BIN" start --id "$SESSION_ID" --env .env 2>&1)
    if ! echo "$output" | grep -q '"status":"running"'; then
        echo "ERROR: Failed to start session: $output" >&2; exit 1
    fi

    # Wait for containers to be ready
    sleep 2
    printf "Session running.\n\n"
}

# --- Tests ---

test_env_vars_are_encrypted() {
    local val
    val=$(container_exec 'echo "$TEST_SECRET"')
    if echo "$val" | grep -q "^ENC\[age:"; then
        log_pass "env vars contain ENC[age:...] tokens"
    else
        log_fail "env vars contain ENC[age:...] tokens" "got: ${val:0:60}"
    fi
}

test_ca_bundle_exists() {
    local result
    result=$(container_exec 'test -f /tmp/airlock-ca-bundle.crt && echo "exists" || echo "missing"')
    if [ "$result" = "exists" ]; then
        log_pass "CA bundle created at /tmp/airlock-ca-bundle.crt"
    else
        log_fail "CA bundle created at /tmp/airlock-ca-bundle.crt" "file not found"
    fi
}

test_ssl_cert_file_set() {
    local val
    val=$(container_exec 'echo "$SSL_CERT_FILE"')
    if [ "$val" = "/tmp/airlock-ca-bundle.crt" ]; then
        log_pass "SSL_CERT_FILE points to combined CA bundle"
    else
        log_fail "SSL_CERT_FILE points to combined CA bundle" "got: $val"
    fi
}

test_header_decryption() {
    local response
    response=$(container_exec 'curl -s https://httpbin.org/headers -H "X-Test-Secret: $TEST_SECRET"')
    if echo "$response" | grep -q '"X-Test-Secret": "hello_from_airlock"'; then
        log_pass "header decryption (ENC -> plaintext)"
    else
        log_fail "header decryption (ENC -> plaintext)" "response: ${response:0:120}"
    fi
}

test_body_decryption() {
    local response
    response=$(container_exec 'curl -s -X POST https://httpbin.org/post -H "Content-Type: application/json" -d "{\"key\": \"$STRIPE_KEY\"}"')
    if echo "$response" | grep -q '"key": "sk_live_test_12345"'; then
        log_pass "body decryption (JSON POST)"
    else
        log_fail "body decryption (JSON POST)" "response: ${response:0:120}"
    fi
}

test_double_quoted_value() {
    local response
    response=$(container_exec 'curl -s https://httpbin.org/headers -H "X-Quoted: $QUOTED_DOUBLE"')
    if echo "$response" | grep -q '"X-Quoted": "double_quoted_value"'; then
        log_pass "double-quoted .env value stripped and decrypted"
    else
        log_fail "double-quoted .env value stripped and decrypted" "response: ${response:0:120}"
    fi
}

test_single_quoted_value() {
    local response
    response=$(container_exec 'curl -s https://httpbin.org/headers -H "X-Quoted: $QUOTED_SINGLE"')
    if echo "$response" | grep -q '"X-Quoted": "single_quoted_value"'; then
        log_pass "single-quoted .env value stripped and decrypted"
    else
        log_fail "single-quoted .env value stripped and decrypted" "response: ${response:0:120}"
    fi
}

test_passthrough_anthropic() {
    local proxy_logs_before proxy_logs_after
    proxy_logs_before=$(docker logs "airlock-proxy-$SESSION_ID" 2>&1 | wc -l)

    # Send a request to anthropic API (passthrough host)
    container_exec 'curl -s -o /dev/null https://api.anthropic.com/v1/messages -H "x-api-key: $TEST_SECRET" -H "content-type: application/json" -H "anthropic-version: 2023-06-01" -d "{\"model\":\"claude-sonnet-4-20250514\",\"max_tokens\":1,\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" 2>/dev/null' || true

    sleep 1
    local log_line
    log_line=$(docker logs "airlock-proxy-$SESSION_ID" 2>&1 | grep '"host": "api.anthropic.com"' | tail -1)
    if echo "$log_line" | grep -q '"action": "passthrough"'; then
        log_pass "Anthropic API traffic passed through (no decryption)"
    else
        log_fail "Anthropic API traffic passed through (no decryption)" "log: $log_line"
    fi
}

test_proxy_log_decrypt_action() {
    local log_line
    log_line=$(docker logs "airlock-proxy-$SESSION_ID" 2>&1 | grep '"action": "decrypt"' | head -1)
    if [ -n "$log_line" ]; then
        log_pass "proxy logs contain decrypt action entries"
    else
        log_fail "proxy logs contain decrypt action entries" "no decrypt log found"
    fi
}

# --- Main ---

main() {
    printf "=== Airlock Proxy E2E Tests ===\n"

    setup

    printf "=== Running Tests ===\n"
    test_env_vars_are_encrypted
    test_ca_bundle_exists
    test_ssl_cert_file_set
    test_header_decryption
    test_body_decryption
    test_double_quoted_value
    test_single_quoted_value
    test_passthrough_anthropic
    test_proxy_log_decrypt_action

    printf "\n=== Results: %d passed, %d failed (of %d) ===\n" "$PASS" "$FAIL" "$((PASS + FAIL))"
    for t in "${TESTS[@]}"; do
        printf "  %s\n" "$t"
    done

    if [ "$FAIL" -gt 0 ]; then
        exit 1
    fi
}

main "$@"
