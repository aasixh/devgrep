#!/usr/bin/env sh
set -eu

repo="devgrep/devgrep"
bin_dir="${BIN_DIR:-$HOME/.local/bin}"
version="${DEVGREP_VERSION:-latest}"
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

mkdir -p "$bin_dir"

if [ "$version" = "latest" ]; then
  url="https://github.com/$repo/releases/latest/download/devgrep_${os}_${arch}.tar.gz"
else
  url="https://github.com/$repo/releases/download/$version/devgrep_${os}_${arch}.tar.gz"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$url" -o "$tmp/devgrep.tar.gz"
tar -xzf "$tmp/devgrep.tar.gz" -C "$tmp"
install -m 0755 "$tmp/devgrep" "$bin_dir/devgrep"

echo "installed devgrep to $bin_dir/devgrep"
