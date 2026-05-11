#!/usr/bin/env bash
# Bootstrap an OpenAI-compatible LLM endpoint on a fresh GPU VM.
# Used by both vast.ai and GCP IaC paths — keep it shell-only and idempotent
# so re-running on an existing host is a no-op.
#
# Configuration (provided via env vars by the caller, or set defaults below):
#   MODEL_NAME    HuggingFace model id, e.g. "meta-llama/Llama-3.1-8B-Instruct"
#   LLM_API_KEY   Bearer token clients must present (also vLLM's --api-key)
#   PORT          Host port to expose (default 8000)
#   GPU_MEM_FRAC  Fraction of GPU memory vLLM may use (default 0.92)
#   HF_TOKEN      Optional HuggingFace token for gated models
#
# After this script finishes, the endpoint is reachable as:
#   POST http://<host>:$PORT/v1/chat/completions   (Bearer $LLM_API_KEY)

set -euo pipefail

MODEL_NAME="${MODEL_NAME:-meta-llama/Llama-3.1-8B-Instruct}"
LLM_API_KEY="${LLM_API_KEY:?LLM_API_KEY is required}"
PORT="${PORT:-8000}"
GPU_MEM_FRAC="${GPU_MEM_FRAC:-0.92}"
HF_TOKEN="${HF_TOKEN:-}"

log() { echo "[llm-server] $*"; }

# --- Docker ---
if ! command -v docker >/dev/null 2>&1; then
  log "Installing Docker…"
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi

# --- NVIDIA Container Toolkit (skip on hosts where it's preinstalled) ---
if ! docker info 2>/dev/null | grep -qi nvidia; then
  log "Installing NVIDIA Container Toolkit…"
  distribution="$(. /etc/os-release && echo "${ID}${VERSION_ID}")"
  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey \
    | gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
  curl -fsSL "https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.list" \
    | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' \
    > /etc/apt/sources.list.d/nvidia-container-toolkit.list
  apt-get update -y
  apt-get install -y nvidia-container-toolkit
  nvidia-ctk runtime configure --runtime=docker
  systemctl restart docker
fi

# --- HuggingFace cache on the host (so model downloads survive container restarts) ---
HF_CACHE="/var/cache/huggingface"
mkdir -p "${HF_CACHE}"

# --- Stop any prior container with our name ---
if docker ps -a --format '{{.Names}}' | grep -qx llm-server; then
  log "Removing previous llm-server container…"
  docker rm -f llm-server >/dev/null
fi

# --- Run vLLM (OpenAI-compatible /v1/* server) ---
log "Starting vLLM serving ${MODEL_NAME} on port ${PORT}…"
HF_ENV=()
if [ -n "${HF_TOKEN}" ]; then
  HF_ENV+=(-e "HUGGING_FACE_HUB_TOKEN=${HF_TOKEN}")
fi

docker run -d \
  --name llm-server \
  --restart unless-stopped \
  --gpus all \
  --ipc=host \
  -p "${PORT}:8000" \
  -v "${HF_CACHE}:/root/.cache/huggingface" \
  "${HF_ENV[@]}" \
  vllm/vllm-openai:latest \
    --model "${MODEL_NAME}" \
    --api-key "${LLM_API_KEY}" \
    --gpu-memory-utilization "${GPU_MEM_FRAC}" \
    --host 0.0.0.0 \
    --port 8000

# --- Smoke-check that the server starts answering ---
log "Waiting for /v1/models to respond (up to 6 minutes for first model download)…"
for i in $(seq 1 72); do
  if curl -fsS -m 4 -H "Authorization: Bearer ${LLM_API_KEY}" \
      "http://127.0.0.1:${PORT}/v1/models" >/dev/null 2>&1; then
    log "Endpoint ready on :${PORT}"
    exit 0
  fi
  sleep 5
done

log "WARN: server did not become healthy within timeout — check 'docker logs llm-server'"
exit 1
