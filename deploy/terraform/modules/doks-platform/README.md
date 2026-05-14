# `doks-platform`

DigitalOcean Kubernetes (DOKS) platform module: VPC + DOKS cluster +
autoscaling node pool + maintenance policy.

## Usage

```hcl
module "platform" {
  source = "../../../modules/doks-platform"

  environment      = "dev"
  cluster_name     = "kubewatcher-dev"
  region           = "sfo3"
  node_size        = "s-2vcpu-2gb"
  node_count_min   = 1
  node_count_max   = 3

  tags = ["kubewatcher", "dev"]
}
```

## Provider

The DigitalOcean provider requires a `DIGITALOCEAN_TOKEN` env var:

```sh
export DIGITALOCEAN_TOKEN="dop_v1_..."
```

## Outputs

- `cluster_name`, `cluster_endpoint`, `cluster_ca_certificate`
- `cluster_id`, `vpc_id`, `ipv4_address`
