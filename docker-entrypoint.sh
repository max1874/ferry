#!/bin/sh
set -eu

listen_host="${FERRY_LISTEN_HOST:-}"
if [ -z "$listen_host" ]; then
    listen_host="$(hostname -i | awk 'NF == 1 { print $1 }')"
    if [ -z "$listen_host" ]; then
        echo "ferry: expected exactly one container IP address; set FERRY_LISTEN_HOST (for example 0.0.0.0) to choose the listener" >&2
        exit 1
    fi
fi

port="${FERRY_PORT:-42817}"
case "$listen_host" in
    *:*) listen_address="[${listen_host}]:${port}" ;;
    *) listen_address="${listen_host}:${port}" ;;
esac
published_host="${FERRY_HOST_IP:-127.0.0.1}"
if [ -n "${FERRY_TRUSTED_ORIGIN:-}" ]; then
    set -- "$@" -trusted-origin "${FERRY_TRUSTED_ORIGIN}"
fi
exec /usr/local/bin/ferry \
    "$@" \
    -lan \
    -listen "${listen_address}" \
    -published-host "${published_host}" \
    -data-dir /data
