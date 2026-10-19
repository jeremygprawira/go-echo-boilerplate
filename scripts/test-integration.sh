#!/usr/bin/env bash
# Runs internal/integration and the cmd/http tests that need a database on a
# throwaway Postgres container. Extra arguments go to go test, as in
# scripts/test-integration.sh -count=2 -shuffle=on.
# The EXIT trap removes the container. A SIGKILL skips the trap and leaks it,
# so remove leftovers with: docker rm -f $(docker ps -qf name=go-echo-integration-)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

if ! docker info >/dev/null 2>&1; then
	echo "make test-integration needs a running Docker daemon."
	echo "Without Docker, point TEST_DATABASE_URL at a database whose name ends in _test and run:"
	echo "  go test -count=1 ./internal/integration/... ./cmd/http/..."
	exit 1
fi

name="go-echo-integration-$$"
trap 'docker rm -f "$name" >/dev/null 2>&1' EXIT
trap 'exit 130' INT TERM
docker run -d --rm --name "$name" -p 127.0.0.1::5432 \
	-e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=app_test \
	postgres:16-alpine >/dev/null

for i in $(seq 150); do
	docker exec "$name" pg_isready -q -U app -d app_test -h 127.0.0.1 && break
	[ "$i" = 150 ] && { echo "postgres did not accept connections within 30s"; exit 1; }
	sleep 0.2
done

addr=$(docker port "$name" 5432/tcp | head -1)
TEST_DATABASE_URL="postgres://app:app@$addr/app_test" go test -count=1 "$@" ./internal/integration/... ./cmd/http/...
