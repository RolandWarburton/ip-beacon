#!/bin/bash
# Detects the primary non-loopback IPv4 address and POSTs to the registry
set -euo pipefail

if [[ -z "${REGISTRY_HOST:-}" ]]; then
  echo "Error: REGISTRY_HOST is not set" >&2
  exit 1
fi

IP=$(ip -4 route get 1 | grep -oP 'src \K\S+')

curl -sf -X POST "https://${REGISTRY_HOST}/register" \
  -H "Content-Type: application/json" \
  -d "{\"host\": \"$(hostname)\", \"ip\": \"$IP\"}"
