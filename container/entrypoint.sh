#!/bin/bash
set -e

# Load encrypted env file if mounted.
# Values are single-quoted by airlock to prevent shell injection.
if [ -f /run/airlock/env.enc ]; then
    set -a
    source /run/airlock/env.enc
    set +a
fi

# Trust the custom CA cert for proxy MITM
if [ -f /usr/local/share/ca-certificates/airlock-proxy.crt ]; then
    export NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/airlock-proxy.crt
fi

exec "$@"
