# Self-hosted LLM infrastructure

KubeWatcher's AI features speak the OpenAI Chat Completions API. By default
they hit DeepSeek's hosted endpoint, but you can point them at any
OpenAI-compatible server. This directory contains IaC for two such backends:

- **`vastai/`** — primary, on-demand spot GPUs from the [vast.ai](https://vast.ai)
  marketplace. Cheap, fast to spin up, no commitments.
- **`gcp/`** — backup / scale-out path on Google Cloud. More expensive but
  comes with everything you'd expect (static IP, IAM, VPC, monitoring).

Both stand up the same workload: a single VM running
[vLLM](https://docs.vllm.ai)'s OpenAI server (`vllm/vllm-openai`), bootstrapped
by the shared script at [`cloud-init/llm-server.sh`](cloud-init/llm-server.sh).

## Layout

```
infra/
├── cloud-init/llm-server.sh   # shared bootstrap (Docker + NVIDIA toolkit + vLLM)
├── vastai/                    # Python orchestrator — vast.ai HTTP API
│   ├── config.example.yaml
│   ├── llm_up.py
│   ├── llm_down.py
│   ├── llm_status.py
│   └── vastai_client.py
└── gcp/                       # Terraform module — google_compute_instance
    ├── main.tf
    ├── variables.tf
    ├── outputs.tf
    └── terraform.tfvars.example
```

## Quick start — vast.ai (primary)

```bash
# One-time setup
cd infra/vastai
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

cp config.example.yaml config.yaml          # edit GPU class, max price, model
export VAST_API_KEY=...                     # https://cloud.vast.ai/account/
export LLM_API_KEY=$(openssl rand -hex 32)  # bearer token clients will present

# Spin up. Prints the endpoint URL when it's ready.
python3 llm_up.py

# Inspect / find the endpoint again later.
python3 llm_status.py

# Tear it down (vast.ai bills until you do).
python3 llm_down.py
```

`llm_up.py` searches the marketplace using the filter in `config.yaml`,
picks the cheapest matching offer, rents it, runs the cloud-init script as
the instance's `onstart`, then waits for the public port to come up.

The endpoint it prints looks like `http://<ip>:<port>`. Add that, plus
`$LLM_API_KEY`, to KubeWatcher's settings (see
**Wiring the app** below).

## Quick start — GCP (backup / scale)

```bash
cd infra/gcp
cp terraform.tfvars.example terraform.tfvars   # set project, IP allowlist, key
terraform init
terraform apply

# Endpoint prints in the outputs:
terraform output endpoint
# → http://34.68.x.y:8000

# Tear down when done.
terraform destroy
```

The default machine is a `g2-standard-8` with one NVIDIA L4 (24 GB VRAM) on
spot pricing — enough for an 8B-class model. For larger models bump
`machine_type`, `gpu_type`, and `gpu_count` in `terraform.tfvars`.

The static IP (`google_compute_address.llm`) survives across `apply`
cycles, so the endpoint URL is stable even if the VM is reprovisioned.

## Wiring the app

vLLM's OpenAI-compatible server can be substituted for DeepSeek without
code changes. In KubeWatcher's **Settings → AI & Integrations** set:

| Field | Value |
|-------|-------|
| LLM API Key | the same `$LLM_API_KEY` you deployed with |
| LLM Base URL | `http://<endpoint-from-iac>:8000/v1` |
| LLM Model | `meta-llama/Llama-3.1-8B-Instruct` (or whatever you served) |

…or via env vars (persisted across restarts):

```bash
export DEEPSEEK_API_KEY=$LLM_API_KEY
export KUBEWATCHER_LLM_BASE_URL=http://<endpoint>:8000/v1
export KUBEWATCHER_LLM_MODEL=meta-llama/Llama-3.1-8B-Instruct
```

Leave them empty to fall back to DeepSeek's hosted API.

## Switching between providers

There's no failover daemon yet — the switch is manual:

1. Update the **LLM Base URL** field in Settings to point at the new
   endpoint, or update `KUBEWATCHER_LLM_BASE_URL`.
2. Click **Apply & Reconnect**. Future inference requests use the new
   endpoint immediately; in-flight ones complete against the old one.

A health-check + auto-failover loop is the obvious next step — see
*Roadmap* below.

## Costs at a glance (rough, mid-2026)

| Stack                     | $/hour | Notes |
|---------------------------|--------|-------|
| vast.ai RTX 4090 (spot)   | $0.20–$0.45 | Marketplace, can be reclaimed |
| GCP g2-standard-8 + L4 (spot) | ~$0.30 | Stable IP, GCP SLA when not spot |
| GCP a2-highgpu-1g + A100 (spot) | ~$1.20 | 40 GB VRAM, fits 30B-class models |

Always tear down when you're done — both backends bill by the second.

## Roadmap

- **Health check + automatic failover.** A small daemon that pings
  `/v1/models` every N seconds and rewrites the app's base URL when the
  primary stops answering.
- **Model preload baked into the image.** vast.ai's first boot has to
  download the model — pre-baking shaves minutes off cold starts.
- **Multi-region GCP.** A managed instance group across two zones, fronted
  by a global L7 LB, would give you HA without manual switching.
