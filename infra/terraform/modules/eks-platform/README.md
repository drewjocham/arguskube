# `eks-platform`

Reusable AWS module: VPC + EKS cluster + managed node group +
spot node group + scheduled scaling.

## Usage

```hcl
module "platform" {
  source = "../../../modules/eks-platform"

  environment  = "dev"
  cluster_name = "kubewatcher-dev"
  cluster_endpoint_public_access = true

  availability_zones   = ["us-west-2a", "us-west-2b"]
  private_subnet_cidrs = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnet_cidrs  = ["10.0.101.0/24", "10.0.102.0/24"]

  node_group_instance_types = ["t3.medium"]
  node_group_desired_size   = 1
  node_group_min_size       = 0
  node_group_max_size       = 3
  use_spot                  = true
  enable_night_scale_in     = true

  tags = local.tags
}
```
