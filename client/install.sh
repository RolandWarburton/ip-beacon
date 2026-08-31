#!/bin/sh
# Installs the beacon client. This script is served by the registry itself,
# which fills in its own address before sending it:
#
#   curl -fsSL https://beacon.example.com/client/install.sh | sudo sh
#
# The files it downloads arrive with that same address already substituted in.
set -eu

BASE_URL="${BASE_URL:-@@BASE_URL@@}"

if [ "$(id -u)" -ne 0 ]; then
  echo "Error: must be run as root" >&2
  exit 1
fi

# An unsubstituted placeholder means this was not fetched from a running registry
case "$BASE_URL" in
  ""|@@*@@)
    echo "Error: fetch this script from a running registry, not from a clone" >&2
    exit 1
    ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

for f in register.sh beacon.service beacon.timer; do
  echo "Downloading $f"
  curl -fsSL "${BASE_URL}/client/$f" -o "${TMP}/$f"
done

install -m 755 "${TMP}/register.sh" /usr/local/bin/beacon-register
install -m 644 "${TMP}/beacon.service" /etc/systemd/system/beacon.service
install -m 644 "${TMP}/beacon.timer" /etc/systemd/system/beacon.timer

systemctl daemon-reload
systemctl enable --now beacon.timer
systemctl start beacon.service

systemctl --no-pager status beacon.timer beacon.service || true
