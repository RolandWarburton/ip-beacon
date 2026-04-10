#!/bin/bash
set -e

# Must be run as root to install systemd units and copy files
if [[ $EUID -ne 0 ]]; then
  echo "Error: must be run as root" >&2
  exit 1
fi

read -rp "Registry host (default: your-server.example.com): " REGISTRY_HOST
REGISTRY_HOST="${REGISTRY_HOST:-your-server.example.com}"

cp "$(dirname "$0")/register.sh" /usr/local/bin/register.sh
chmod +x /usr/local/bin/register.sh

# Install the service unit, substituting in the chosen registry host
sed "s/REGISTRY_HOST=your-server.example.com/REGISTRY_HOST=${REGISTRY_HOST}/" \
  "$(dirname "$0")/ip-beacon.service" \
  > /etc/systemd/system/ip-beacon.service

# Install the timer unit
cp "$(dirname "$0")/ip-beacon.timer" /etc/systemd/system/ip-beacon.timer

systemctl daemon-reload

# Stop any currently running instance before enabling the timer
systemctl stop ip-beacon.service 2>/dev/null || true
systemctl enable --now ip-beacon.timer

systemctl status ip-beacon.timer
systemctl status ip-beacon.service
