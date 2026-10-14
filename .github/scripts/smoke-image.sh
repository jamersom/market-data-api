#!/usr/bin/env bash
set -euo pipefail

# Only disposable resources created here are touched. No market data or migrations.
image=${1:?Usage: smoke-image.sh LOCAL_IMAGE_OR_DIGEST}
image_id=$(docker image inspect --format '{{.Id}}' "$image")
suffix="${RANDOM}-$$"
network="market-api-smoke-${suffix}"
database="market-api-smoke-db-${suffix}"
api="market-api-smoke-api-${suffix}"
network_id=
database_id=
api_id=

cleanup() {
  if [[ -n "$api_id" ]]; then docker rm -fv "$api_id" >/dev/null 2>&1 || true; fi
  if [[ -n "$database_id" ]]; then docker rm -fv "$database_id" >/dev/null 2>&1 || true; fi
  if [[ -n "$network_id" ]]; then docker network rm "$network_id" >/dev/null 2>&1 || true; fi
}
trap cleanup EXIT

network_id=$(docker network create "$network")
# Trust is restricted to this temporary bridge; PostgreSQL has no published port.
database_id=$(docker run -d --name "$database" --network "$network" --network-alias health-db \
  -e POSTGRES_HOST_AUTH_METHOD=trust postgres:17-alpine)

database_ready=false
for ((attempt=0; attempt<60; attempt++)); do
  if docker exec "$database" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1; then
    database_ready=true
    break
  fi
  sleep 1
done
if [[ "$database_ready" != true ]]; then
  echo 'Disposable PostgreSQL did not become ready' >&2
  exit 1
fi

api_id=$(docker run -d --name "$api" --network "$network" -p 127.0.0.1::8080 \
  -e DATABASE_URL='postgres://postgres@health-db:5432/postgres?sslmode=disable' \
  -e SERVER_ADDRESS=:8080 -e HEALTH_CHECK_TIMEOUT=2s \
  "$image_id")
address=$(docker port "$api" 8080/tcp)

expect_http() {
  local path=$1 expected=$2 body=$3 response attempt http_status response_body curl_exit=0
  for ((attempt=0; attempt<30; attempt++)); do
    http_status=
    # A matching body/status is usable only after a successful transfer.
    if response=$(curl --silent --max-time 5 --write-out '\n%{http_code}' "http://${address}${path}"); then
      curl_exit=0
      http_status=${response##*$'\n'}
      response_body=${response%$'\n'*}
      if [[ "$http_status" == "$expected" ]] && [[ "$response_body" == "${body}"$'\n' ]]; then
        printf '%s: %s\n' "$path" "$expected"
        return 0
      fi
    else
      curl_exit=$?
    fi
    sleep 1
  done
  printf 'Expected %s from %s was not observed (curl exit=%s, HTTP=%s)\n' \
    "$expected" "$path" "$curl_exit" "${http_status:-unavailable}" >&2
  return 1
}

expect_http /health/live 200 '{"status":"ok"}'
expect_http /health/ready 200 '{"status":"ok"}'

# Also exercise the HEALTHCHECK baked into the exact candidate image.
healthy=false
for ((attempt=0; attempt<120; attempt++)); do
  if [[ "$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "$api")" == healthy ]]; then
    healthy=true
    break
  fi
  sleep 1
done
if [[ "$healthy" != true ]]; then
  echo 'Candidate image did not become healthy' >&2
  exit 1
fi

# Disconnect only our disposable database, preserving HTTP access to the API.
docker network disconnect "$network" "$database"
expect_http /health/ready 503 '{"status":"unavailable"}'
expect_http /health/live 200 '{"status":"ok"}'
docker network connect --alias health-db "$network" "$database"
expect_http /health/ready 200 '{"status":"ok"}'
echo "Operational smoke test passed for ${image_id}"
