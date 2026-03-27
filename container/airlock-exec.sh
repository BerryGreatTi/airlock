#!/bin/bash
# Wrapper for docker exec that loads airlock environment.
# Usage: docker exec <container> airlock-exec.sh <command> [args...]
source /home/airlock/.airlock-env.sh 2>/dev/null
exec "$@"
