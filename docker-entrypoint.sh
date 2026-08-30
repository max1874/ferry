#!/bin/sh
set -eu

container_ip="$(hostname -i | awk 'NF == 1 { print $1 }')"
if [ -z "$container_ip" ]; then
    echo "ferry: expected exactly one container IP address" >&2
    exit 1
fi

port="${FERRY_PORT:-42817}"
published_host="${FERRY_HOST_IP:-127.0.0.1}"
exec /usr/local/bin/ferry \
    "$@" \
    -lan \
    -listen "${container_ip}:${port}" \
    -published-host "${published_host}" \
    -data-dir /data
