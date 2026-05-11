# `gke-platform`

Reusable GCP module: VPC + Cloud NAT + GKE cluster + node pool + required
APIs + node service account + optional scheduled scaling.

Environment-agnostic — pass `environment` and sizing and it self-tunes.

## Usage

```hcl
module "platform" {
  source = "../../../modules/gke-platform"

  project_id  = "my-gcp-project"
  environment = "dev"
  cluster_name = "kubewatcher-dev"

  enable_private_endpoint = false

  machine_type    = "e2-medium"
  node_count_min  = 1
  node_count_max  = 3
  enable_night_scale_in = true

  tags = local.tags
}
```

## Outputs

- `cluster_name`, `cluster_endpoint`, `cluster_ca_certificate`
- `cluster_location`, `vpc_name`, `subnet_name`
- `node_service_account`
