# GCP LLM (backup / scale)

A small Terraform module that stands up one GPU VM running vLLM behind a
static external IP, in your existing VPC. Same cloud-init script as the
vast.ai path, so the workload is identical.

## Prereqs

- `terraform >= 1.6` (`brew install terraform`)
- `gcloud` authed to a project: `gcloud auth application-default login`
- The Compute Engine API enabled in the project
- A GPU quota in the chosen region (request via Console → IAM → Quotas)

## Apply

```bash
cd infra/gcp
cp terraform.tfvars.example terraform.tfvars
$EDITOR terraform.tfvars                   # set project_id, llm_api_key, allowed CIDRs

terraform init
terraform plan                             # sanity-check before money is spent
terraform apply

# Endpoint URL:
terraform output endpoint
# → http://34.68.x.y:8000
```

The static external IP is a separate resource (`google_compute_address`)
that survives VM recreates — the endpoint URL is stable.

## What it provisions

- `google_compute_address.llm` — static external IP
- `google_compute_firewall.llm_ingress` — allows TCP/22 + the LLM port
  from the CIDRs in `allowed_source_ranges`
- `google_compute_instance.llm` — the actual VM

Default machine: `g2-standard-8` (8 vCPU / 32 GB RAM) + 1× NVIDIA L4
(24 GB VRAM), spot pricing, NVIDIA Deep Learning VM image so CUDA is
preinstalled. The `metadata_startup_script` inlines the cloud-init script
plus the env vars vLLM needs.

## Sizing for bigger models

| Model | Recommended GPU | Variables |
|-------|-----------------|-----------|
| 7–8B  | L4 (24 GB)      | defaults  |
| 13B   | A100 40 GB      | `machine_type = "a2-highgpu-1g"`, `gpu_type = "nvidia-tesla-a100"` |
| 30B   | A100 80 GB      | `gpu_type = "nvidia-a100-80gb"` |
| 70B   | 2× A100 80 GB or H100 | `gpu_count = 2`, or `gpu_type = "nvidia-h100-80gb"` |

Boot disk size scales with model weight + KV cache; bump `disk_gb` if you
serve multiple models from the same host.

## Spot vs. on-demand

`preemptible = true` (the default) maps to a `SPOT` provisioning model.
GCP can reclaim the VM with 30 s notice; vLLM restarts cleanly via Docker
`--restart unless-stopped` once the host comes back. For an SLA-bound
production endpoint set `preemptible = false`.

## Tear down

```bash
terraform destroy
```

This removes the VM, firewall rule, and static IP. Confirm `terraform
state list` is empty afterwards if you want to make sure there's no
lingering billable resource.
