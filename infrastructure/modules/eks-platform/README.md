# `eks-platform`

Reusable AWS module: VPC + cluster security group + EKS cluster +
managed node group + cluster add-ons + optional scheduled scaling.

This module is environment-agnostic — pass in the environment name and
sizing and it will tune itself (NAT gateway count, public-API access,
scaling schedule). It does **not** install Helm releases; pair it with
`kubewatcher-helm` for that.

## Usage

```hcl
module "platform" {
  source = "../../../modules/eks-platform"

  environment           = "dev"
  cluster_name          = "kubewatcher-dev"
  cluster_version       = "1.30"
  cluster_endpoint_public_access = true

  availability_zones    = ["us-west-2a", "us-west-2b", "us-west-2c"]
  private_subnet_cidrs  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  public_subnet_cidrs   = ["10.0.101.0/24", "10.0.102.0/24", "10.0.103.0/24"]

  node_group_instance_types = ["t3.medium"]
  node_group_desired_size   = 2
  node_group_min_size       = 0
  node_group_max_size       = 6
  enable_night_scale_in     = true

  tags = local.tags
}
```

## Outputs

- `cluster_name`, `cluster_endpoint`, `cluster_certificate_authority_data`
- `cluster_security_group_id`, `oidc_provider_arn`
- `vpc_id`, `private_subnet_ids`, `public_subnet_ids`
- `node_group` — full managed node group object
