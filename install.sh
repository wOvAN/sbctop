#!/usr/bin/env bash
# One-line install of the latest rkmon release:
#   curl -fsSL https://raw.githubusercontent.com/wOvAN/rkmon/main/install.sh | sudo bash
set -euo pipefail

REPO="wOvAN/rkmon"
DEST="${DEST:-/usr/local/bin}"

[ "$(uname -s)-$(uname -m)" = "Linux-aarch64" ] || { echo "rkmon: Linux/aarch64 only" >&2; exit 1; }

url=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
	sed -n 's/.*"browser_download_url": *"\([^"]*rkmon-arm64\)".*/\1/p')
[ -n "$url" ] || { echo "rkmon: no release asset found" >&2; exit 1; }

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp"
install -Dm 755 "$tmp" "$DEST/rkmon"
echo "installed $DEST/rkmon $("${DEST}/rkmon" -v)"
