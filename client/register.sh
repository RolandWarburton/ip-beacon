#!/bin/bash
# Detects the primary non-loopback IPv4 address and POSTs to the registry
SERVER_HOST="${REGISTRY_HOST:-your-server.example.com}"
IP=$(ip -4 route get 1 | grep -oP '\''src \K\S+'\'')

curl -s -X POST "https://${SERVER_HOST}/register" \
  -H "Content-Type: application/json" \
  -d "{\"host\": \"$(hostname)\", \"ip\": \"$IP\"}"
