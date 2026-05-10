# vast.ai LLM orchestrator

Three Python scripts that talk to vast.ai's REST API directly:

| Script | What it does |
|--------|--------------|
| `llm_up.py` | Search for an offer matching `config.yaml`, rent it, run cloud-init, wait for the public port, print the endpoint. |
| `llm_status.py` | List every instance tagged `kube-watcher-llm` (or `--all`) with its endpoint URL. |
| `llm_down.py` | Destroy every tagged instance (or specific ids). |

`vastai_client.py` is the shared client + helpers; it has no `__main__`.

## Setup

```bash
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

cp config.example.yaml config.yaml   # edit GPU class, max bid, model, …
export VAST_API_KEY=...              # https://cloud.vast.ai/account/
export LLM_API_KEY=$(openssl rand -hex 32)   # bearer token for clients
```

## Why HTTP API, not the `vastai` CLI

The vast.ai Python SDK shells out to its own CLI internally and its surface
churns between releases. The HTTP API at `console.vast.ai/api/v0/` is
documented and stable — `vastai_client.py` is ~150 lines of `requests` calls.

## How `config.yaml` maps onto a marketplace search

The fields under `gpu:` and `host:` translate into vast.ai's filter language
(see `build_offer_query` in `vastai_client.py`). The default config asks for:

- 1 GPU from the {RTX 4090, RTX 3090, A6000} set
- ≥24 GB total VRAM
- ≤ $0.50/hour all-in
- Verified host, ≥0.95 reliability
- ≥60 GB disk, ≥200 Mbps download

Relax any of these if `llm_up.py --dry-run` returns no offers.

## Tag-based ownership

Every instance gets `label: kube-watcher-llm` (overridable via `tag:` in
`config.yaml`). `llm_down.py` and `llm_status.py` filter on this label so
they never touch instances you spun up for other purposes. Pass `--all` to
override.

## Endpoint shape

The cloud-init bootstrap script runs vLLM with `--api-key $LLM_API_KEY`. The
endpoint is OpenAI-compatible:

```
POST http://<ip>:<port>/v1/chat/completions
Authorization: Bearer $LLM_API_KEY
Content-Type: application/json

{ "model": "meta-llama/Llama-3.1-8B-Instruct",
  "messages": [{"role": "user", "content": "ping"}] }
```

Wire it into KubeWatcher via Settings (LLM Base URL = `http://<ip>:<port>/v1`,
LLM Model = whatever you served).

## Cost discipline

vast.ai bills by the second. **Always** tear down with `llm_down.py` when
you're done, or set a budget alarm. `llm_status.py` is a useful safety
check before logging off.
