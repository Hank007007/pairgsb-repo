#!/usr/bin/env bash
# Local walkthrough for the request-shape pipeline. Starts gatewayd on a
# throwaway port, posts a handful of bodies, prints status + response, and
# stops the server.
#
# Bodies are sent as percent-encoded form data so the script behaves the same
# regardless of the terminal's active code page.
set -u

ROOT="$(cd "$(dirname "$0")" && pwd)"
PORT="${PORT:-18080}"
BASE="http://127.0.0.1:${PORT}"

cd "$ROOT" || exit 1
go build -o ./gatewayd ./cmd/gatewayd || exit 1
./gatewayd -addr "127.0.0.1:${PORT}" >/dev/null 2>&1 &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null; rm -f ./gatewayd' EXIT

for _ in $(seq 1 50); do
  if curl -s -o /dev/null "${BASE}/profile.update"; then
    break
  fi
  sleep 0.1
done

req() {
  local label="$1"
  local path="$2"
  shift 2
  echo "=== ${label}"
  curl -s -X POST "${BASE}${path}" "$@" -w '\nHTTP %{http_code}\n'
  echo
}

BASE_FIELDS='username=ada&display_name=Ada&owner.email=ada%40example.com'

echo "## integers: target width and range"
req "attempts=3 (in range)" /account.create --data "${BASE_FIELDS}&enabled=true&attempts=3"
req "json attempts=2147483648" /account.create \
  -H 'Content-Type: application/json' \
  --data '{"username":"ada","display_name":"Ada","owner.email":"ada@example.com","enabled":true,"attempts":2147483648}'
req "attempts=4294967297" /account.create --data "${BASE_FIELDS}&enabled=true&attempts=4294967297"

echo "## booleans: accepted literals"
req "enabled=1" /account.create --data "${BASE_FIELDS}&attempts=1&enabled=1"
req "enabled=TRUE" /account.create --data "${BASE_FIELDS}&attempts=1&enabled=TRUE"
req "enabled=on" /account.create --data "${BASE_FIELDS}&attempts=1&enabled=on"

echo "## repeated assignment and ambiguous paths"
req "form items[0] beside items[]" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&items%5B0%5D=go&items%5B%5D=api"
req "form duplicate username" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&username=mallory"
req 'json literal "owner.email" beside nested owner.email' /account.create \
  -H 'Content-Type: application/json' \
  --data '{"username":"ada","display_name":"Ada","attempts":1,"enabled":true,"owner.email":"literal@example.com","owner":{"email":"nested@example.com"}}'
req "nesting deeper than account.create allows" /account.create \
  -H 'Content-Type: application/json' \
  --data '{"username":"ada","display_name":"Ada","attempts":1,"enabled":true,"owner.email":"ada@example.com","a":{"b":{"c":{"d":"deep"}}}}'

echo "## character repertoire"
req "safe field with fullwidth angle brackets" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&username=%EF%BC%9Cscript%EF%BC%9E"
req "safe field with a combining mark" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&username=cafe%CC%81"
req "value that grows when normalized" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&company=%E3%8D%BF%E3%8D%BF%E3%8D%BF"

echo "## byte limits"
req "CJK over the byte cap" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&company=%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD"
req "emoji over the byte cap" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&company=%F0%9F%98%80%F0%9F%98%80%F0%9F%98%80"
req "mixed text longer than the byte cap" /account.create \
  --data "${BASE_FIELDS}&attempts=1&enabled=true&company=a%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD%E4%B8%AD"

echo "## rejection payload"
req "attempts=abc" /account.create --data "${BASE_FIELDS}&enabled=true&attempts=abc"
