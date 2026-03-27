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

exec "$@"
