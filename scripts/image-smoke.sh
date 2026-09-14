#!/bin/sh
# Deployment journey for a Ferry image, run on a disposable CI host. It uses
# the real compose.yaml, publishes on 127.0.0.1:42817 and deletes its volumes.
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$repo_root"

BASE_URL="http://127.0.0.1:42817"
SMOKE_TEXT="ferry image smoke $$"
SMOKE_PASSWORD="ferry-smoke-password"
PROJECT=
TOKEN=
WORK=$(mktemp -d "${TMPDIR:-/tmp}/ferry-image-smoke.XXXXXX")

usage() {
    cat >&2 <<'EOF'
Usage:
  scripts/image-smoke.sh run IMAGE PLATFORM   start IMAGE for PLATFORM, restart it, keep data
  scripts/image-smoke.sh upgrade IMAGE        switch a source-built deployment to IMAGE, keep data
EOF
    exit 2
}

fail() {
    echo "image-smoke: $*" >&2
    exit 1
}

cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if [ -n "$PROJECT" ]; then
        [ "$status" -eq 0 ] || docker compose -p "$PROJECT" logs --no-color ferry >&2 || true
        docker compose -p "$PROJECT" down --volumes --remove-orphans >/dev/null 2>&1 || true
    fi
    rm -rf "$WORK"
    exit "$status"
}

wait_healthy() {
    attempts=0
    until curl -fsS "$BASE_URL/healthz" >/dev/null 2>&1; do
        attempts=$((attempts + 1))
        [ "$attempts" -lt 120 ] || fail "Ferry did not become healthy at $BASE_URL"
        sleep 1
    done
}

api() {
    method=$1
    path=$2
    shift 2
    curl -fsS -X "$method" -H "Authorization: Bearer $TOKEN" "$@" "$BASE_URL$path"
}

seed() {
    TOKEN=$(curl -fsS -X POST -H 'Content-Type: application/json' \
        -d '{"device_name":"Image smoke","password":""}' "$BASE_URL/api/v1/access/join" | jq -r .token)
    [ -n "$TOKEN" ] && [ "$TOKEN" != null ] || fail "join returned no device token"
    api POST /api/v1/messages/text -H 'Content-Type: application/json' \
        -d "$(jq -n --arg text "$SMOKE_TEXT" '{text: $text}')" >/dev/null
    printf 'ferry image smoke file\n' >"$WORK/smoke.txt"
    api POST /api/v1/messages/file -F "file=@$WORK/smoke.txt;type=text/plain" >/dev/null
    api PUT /api/v1/settings/access -H 'Content-Type: application/json' \
        -d "{\"password\":\"$SMOKE_PASSWORD\"}" >/dev/null
}

verify() {
    api GET /api/v1/session >/dev/null || fail "device identity did not survive"
    curl -fsS "$BASE_URL/api/v1/access" | jq -e '.password_required == true' >/dev/null ||
        fail "access password did not survive"
    status=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' \
        -d '{"device_name":"No password","password":""}' "$BASE_URL/api/v1/access/join")
    [ "$status" = 401 ] || fail "a device joined without the saved password (HTTP $status)"
    api GET "/api/v1/messages?after=0&limit=200" >"$WORK/messages.json"
    jq -e --arg text "$SMOKE_TEXT" '[.messages[] | select(.kind == "text" and .text == $text)] | length == 1' \
        "$WORK/messages.json" >/dev/null || fail "text message did not survive"
    download=$(jq -r '[.messages[] | select(.kind == "file" and .file.name == "smoke.txt")][0].file.download_url' "$WORK/messages.json")
    [ -n "$download" ] && [ "$download" != null ] || fail "file message did not survive"
    api GET "$download" -o "$WORK/downloaded.txt"
    cmp -s "$WORK/smoke.txt" "$WORK/downloaded.txt" || fail "downloaded file bytes differ"
    curl -fsS "$BASE_URL/" | grep -Fq 'id="connect-qr"' || fail "Web app is missing the connect section"
}

running_platform() {
    container=$(docker compose -p "$PROJECT" ps -q ferry)
    image_id=$(docker inspect --format '{{.Image}}' "$container")
    docker image inspect --format '{{.Os}}/{{.Architecture}}' "$image_id"
}

run_image() {
    image=$1
    platform=$2
    PROJECT="ferry-smoke-$(printf '%s' "$platform" | tr '/' '-')"
    unset COMPOSE_FILE
    FERRY_IMAGE=$image
    DOCKER_DEFAULT_PLATFORM=$platform
    export FERRY_IMAGE DOCKER_DEFAULT_PLATFORM
    docker compose -p "$PROJECT" up -d --pull always --quiet-pull
    [ "$(running_platform)" = "$platform" ] || fail "running image is $(running_platform), expected $platform"
    wait_healthy
    logs=$(docker compose -p "$PROJECT" logs --no-color ferry)
    printf '%s\n' "$logs" | grep -Fq "listening on " || fail "startup log does not name the listener"
    printf '%s\n' "$logs" | grep -Fq "open Ferry at http://127.0.0.1:42817 on this computer only" ||
        fail "startup log does not name the browser address"
    seed
    docker compose -p "$PROJECT" restart ferry
    wait_healthy
    verify
    docker compose -p "$PROJECT" down --volumes --rmi all
    PROJECT=
    echo "image smoke passed for $platform"
}

upgrade_to_image() {
    image=$1
    PROJECT=ferry-upgrade-smoke
    unset DOCKER_DEFAULT_PLATFORM
    COMPOSE_FILE="$repo_root/compose.yaml:$repo_root/compose.build.yaml"
    export COMPOSE_FILE
    docker compose -p "$PROJECT" up -d --build
    wait_healthy
    seed
    # Keep the named volume: this is the upgrade a source deployment performs.
    docker compose -p "$PROJECT" down
    COMPOSE_FILE="$repo_root/compose.yaml"
    FERRY_IMAGE=$image
    export COMPOSE_FILE FERRY_IMAGE
    docker compose -p "$PROJECT" up -d --pull always --quiet-pull
    wait_healthy
    verify
    docker compose -p "$PROJECT" down --volumes
    PROJECT=
    echo "source-to-image upgrade smoke passed"
}

trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

case "${1:-}" in
    run)
        [ "$#" -eq 3 ] || usage
        run_image "$2" "$3"
        ;;
    upgrade)
        [ "$#" -eq 2 ] || usage
        upgrade_to_image "$2"
        ;;
    *) usage ;;
esac
