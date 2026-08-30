#!/bin/sh
set -eu

container_ip="$(hostname -i | awk 'NF == 1 { print $1 }')"
if [ -z "$container_ip" ]; then
    echo "ferry: expected exactly one container IP address" >&2
    exit 1
fi

port="${FERRY_PORT:-42817}"
exec /usr/local/bin/ferry \
    -lan \
    -listen "${container_ip}:${port}" \
    -data-dir /data \
    "$@"
