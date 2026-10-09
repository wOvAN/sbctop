#!/usr/bin/env bash
# One-line install of the latest sbctop release:
#   curl -fsSL https://raw.githubusercontent.com/wOvAN/sbctop/main/install.sh | sudo bash
set -euo pipefail

REPO="wOvAN/sbctop"
DEST="${DEST:-/usr/local/bin}"

[ "$(uname -s)-$(uname -m)" = "Linux-aarch64" ] || { echo "sbctop: Linux/aarch64 only" >&2; exit 1; }

url=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
	sed -n 's/.*"browser_download_url": *"\([^"]*sbctop-arm64\)".*/\1/p')
[ -n "$url" ] || { echo "sbctop: no release asset found" >&2; exit 1; }

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp"
install -Dm 755 "$tmp" "$DEST/sbctop"
echo "installed $DEST/sbctop $("${DEST}/sbctop" -v)"
