#!/bin/bash
set -e

# Load encrypted env file if mounted.
# Values are single-quoted by airlock to prevent shell injection.
if [ -f /run/airlock/env.enc ]; then
    set -a
    source /run/airlock/env.enc
    set +a
fi

# Trust the custom CA cert for proxy MITM.
# Builds a combined CA bundle so curl, python, and other tools trust the proxy.
if [ -f /usr/local/share/ca-certificates/airlock-proxy.crt ]; then
    COMBINED_CA="/tmp/airlock-ca-bundle.crt"
    cat /etc/ssl/certs/ca-certificates.crt /usr/local/share/ca-certificates/airlock-proxy.crt > "$COMBINED_CA"
    export SSL_CERT_FILE="$COMBINED_CA"
    export REQUESTS_CA_BUNDLE="$COMBINED_CA"
    export CURL_CA_BUNDLE="$COMBINED_CA"
    export NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/airlock-proxy.crt
fi

# Write environment setup to .bashrc so docker exec sessions inherit env vars.
# PID 1 env is NOT inherited by docker exec, so this is required.
cat > /home/airlock/.airlock-env.sh << 'ENVEOF'
export LANG=C.UTF-8

if [ -f /run/airlock/env.enc ]; then
    set -a
    source /run/airlock/env.enc
    set +a
fi
if [ -f /usr/local/share/ca-certificates/airlock-proxy.crt ]; then
    export SSL_CERT_FILE="/tmp/airlock-ca-bundle.crt"
    export REQUESTS_CA_BUNDLE="/tmp/airlock-ca-bundle.crt"
    export CURL_CA_BUNDLE="/tmp/airlock-ca-bundle.crt"
    export NODE_EXTRA_CA_CERTS=/usr/local/share/ca-certificates/airlock-proxy.crt
fi
ENVEOF

# Source from .bashrc for interactive shells
grep -q 'airlock-env.sh' /home/airlock/.bashrc 2>/dev/null || \
    echo 'source /home/airlock/.airlock-env.sh' >> /home/airlock/.bashrc

exec tail -f /dev/null
