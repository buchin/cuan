#!/usr/bin/env bash
set -euo pipefail
export PATH="$HOME/.local/bin:$HOME/.local/go/bin:$PATH"
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/cuan-s25"
if ! command -v go >/dev/null; then
  curl --location --fail --silent --show-error --max-time 120 https://go.dev/dl/go1.27.0.linux-amd64.tar.gz -o "$HOME/.local/go-runtime.tar.gz"
  tar -C "$HOME/.local" -xzf "$HOME/.local/go-runtime.tar.gz"
  rm "$HOME/.local/go-runtime.tar.gz"
fi
GOMAXPROCS=2 CGO_ENABLED=0 go build -p 2 -o "$HOME/.local/share/cuan-s25/cuan-server" ./cmd/server
python3 scripts/install-antigravity.py
if [ -d staging-private ]; then
  chmod 700 staging-private
  for name in database.json ssh.json tunnel-key known_hosts; do
    install -m 600 "staging-private/$name" "$HOME/.local/share/cuan-s25/$name"
  done
  rm -rf staging-private
fi
exec python3 scripts/run-s25-staging.py
