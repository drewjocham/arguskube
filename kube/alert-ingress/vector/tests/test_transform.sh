#!/usr/bin/env bash
set -euo pipefail

DIR="$(cd "$(dirname "$0")/.." && pwd)"
VECTOR="${VECTOR_BIN:-vector}"

echo "Validating alert-ingress Vector config..."
"$VECTOR" validate "$DIR/base.toml" --no-environment

echo "Validating agent-sidecar Vector config..."
"$VECTOR" validate "$DIR/agent-sidecar.toml" --no-environment

echo "All Vector configs valid."
