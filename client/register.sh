#!/bin/sh
# Detects the primary non-loopback IPv4 address and POSTs it to the registry.
set -eu

REGISTRY_URL="${REGISTRY_URL:-@@BASE_URL@@}"

case "$REGISTRY_URL" in
  ""|@@*@@)
    echo "Error: REGISTRY_URL is not set" >&2
    exit 1
    ;;
esac

# Ask the routing table which source address would be used to reach the wider
# network. That is the address other hosts on the LAN can actually reach.
IP=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<NF; i++) if ($i == "src") {print $(i+1); exit}}')

if [ -z "$IP" ]; then
  echo "Error: could not determine local IPv4 address" >&2
  exit 1
fi

exec curl -sf -X POST "${REGISTRY_URL%/}/register" \
  -H "Content-Type: application/json" \
  -d "{\"host\": \"$(hostname)\", \"ip\": \"${IP}\"}"
