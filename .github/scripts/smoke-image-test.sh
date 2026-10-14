#!/usr/bin/env bash
set -euo pipefail

# Regression tests for the script's control flow, not Docker/PostgreSQL integration.
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
test_dir=$(mktemp -d)
export SMOKE_TEST_TRACE="$test_dir/calls"
export SMOKE_TEST_ATTEMPTS="$test_dir/attempts"
cleanup_test() {
  rm -f "$SMOKE_TEST_TRACE" "$SMOKE_TEST_ATTEMPTS"
  rmdir "$test_dir"
}
trap cleanup_test EXIT

# Functions are exported only to the child Bash; no real Docker or HTTP calls.
docker() {
  case "$1 $2" in
    'image inspect') echo sha256:smoke-test-image ;;
    'network create') echo smoke-test-network ;;
    'run -d')
      if [[ "$*" == *postgres:17-alpine* ]]; then
        if [[ "$SMOKE_TEST_CASE" == database_creation_failure ]]; then return 1; fi
        echo smoke-test-database
      else
        [[ "${*: -1}" == sha256:smoke-test-image ]] || return 1
        echo smoke-test-api
      fi ;;
    'exec '*) return 0 ;;
    'port '*) echo 127.0.0.1:18080 ;;
    'inspect --format') echo healthy ;;
    'network disconnect')
      export SMOKE_TEST_DATABASE=unavailable
      echo disconnect >> "$SMOKE_TEST_TRACE" ;;
    'network connect')
      export SMOKE_TEST_DATABASE=recovered
      echo reconnect >> "$SMOKE_TEST_TRACE" ;;
    'rm -fv')
      case "$3" in
        smoke-test-api|smoke-test-database) echo "remove $3" >> "$SMOKE_TEST_TRACE" ;;
        *) return 1 ;;
      esac ;;
    'network rm')
      [[ "$3" == smoke-test-network ]] || return 1
      echo 'remove smoke-test-network' >> "$SMOKE_TEST_TRACE" ;;
    *) echo "Unexpected mocked Docker command: $1 $2" >&2; return 1 ;;
  esac
}

curl() {
  local path=${*: -1} status=200 body='{"status":"ok"}' exit_code=0 attempts write_out=
  while (($#)); do
    if [[ "$1" == --write-out ]]; then write_out=$2; break; fi
    shift
  done
  if [[ "$path" == */health/ready && "$SMOKE_TEST_DATABASE" == unavailable ]]; then
    status=503
    body='{"status":"unavailable"}'
  fi
  case "$SMOKE_TEST_CASE" in
    http_404) status=404 ;;
    timeout) exit_code=28 ;;
    connection_failure) exit_code=7 ;;
    dns_failure) exit_code=6 ;;
    incomplete_transfer) exit_code=18 ;;
    bad_body) body='{"status":"unexpected"}' ;;
    readiness_not_ready)
      if [[ "$path" == */health/ready && "$SMOKE_TEST_DATABASE" == available ]]; then
        status=503; body='{"status":"unavailable"}'
      fi ;;
    outage_not_detected)
      if [[ "$SMOKE_TEST_DATABASE" == unavailable ]]; then status=200; body='{"status":"ok"}'; fi ;;
    recovery_failure)
      if [[ "$path" == */health/ready && "$SMOKE_TEST_DATABASE" == recovered ]]; then
        status=503; body='{"status":"unavailable"}'
      fi ;;
    transient_failure)
      attempts=$(< "$SMOKE_TEST_ATTEMPTS")
      printf '%s\n' "$((attempts + 1))" > "$SMOKE_TEST_ATTEMPTS"
      if ((attempts < 2)); then exit_code=7; fi ;;
  esac
  # Deliberately return a complete, matching payload even on transport failure.
  printf '%s\n' "$body"
  case "$write_out" in
    '\n%{http_code}') printf '\n%s' "$status" ;;
    '%{http_code}') printf '%s' "$status" ;;
    *) return 2 ;;
  esac
  echo "curl $status exit=$exit_code" >> "$SMOKE_TEST_TRACE"
  return "$exit_code"
}
sleep() { :; }
export -f docker curl sleep

run_case() {
  local name=$1 expected=$2 output result=0
  : > "$SMOKE_TEST_TRACE"
  echo 0 > "$SMOKE_TEST_ATTEMPTS"
  if output=$(SMOKE_TEST_CASE="$name" SMOKE_TEST_DATABASE=available \
    bash "$script_dir/smoke-image.sh" smoke-test-reference 2>&1); then
    result=0
  else
    result=$?
  fi
  if [[ "$expected" == success && "$result" != 0 ]] || [[ "$expected" == failure && "$result" == 0 ]]; then
    printf 'FAIL %s: exit=%s\n%s\n' "$name" "$result" "$output" >&2
    return 1
  fi
  if [[ "$name" == database_creation_failure ]]; then
    [[ "$(< "$SMOKE_TEST_TRACE")" == 'remove smoke-test-network' ]]
  else
    for resource in smoke-test-api smoke-test-database smoke-test-network; do
      [[ "$(grep -c "^remove $resource$" "$SMOKE_TEST_TRACE")" == 1 ]]
    done
  fi
  if [[ "$expected" == success ]]; then
    [[ "$output" == *'/health/ready: 503'* && "$output" == *'Operational smoke test passed'* ]]
    [[ "$(grep -c '^disconnect$' "$SMOKE_TEST_TRACE")" == 1 ]]
    [[ "$(grep -c '^reconnect$' "$SMOKE_TEST_TRACE")" == 1 ]]
    [[ "$(grep -c '^curl 200 exit=0$' "$SMOKE_TEST_TRACE")" == 4 ]]
  fi
  if [[ "$name" == transient_failure ]]; then
    [[ "$(grep -c '^curl 200 exit=7$' "$SMOKE_TEST_TRACE")" == 2 ]]
  fi
  printf 'PASS %s (mocked Docker/HTTP)\n' "$name"
}

run_case success success
run_case transient_failure success
for scenario in http_404 timeout connection_failure dns_failure incomplete_transfer bad_body \
  readiness_not_ready outage_not_detected recovery_failure database_creation_failure; do
  run_case "$scenario" failure
done
echo '12 script regression cases passed; no real containers or PostgreSQL were used.'
