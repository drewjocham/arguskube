# `argus-helm`

Reusable module that installs every Argus Helm release into a
single namespace. Each release is independently togglable; resource
sizing flows from a single `environment` variable so dev / staging /
prod stay consistent.

The module assumes the Helm + Kubernetes providers are already
configured by the caller (typically against an EKS cluster output by
`eks-platform`).

## Usage

```hcl
module "apps" {
  source = "../../../modules/argus-helm"

  environment = "dev"
  chart_path  = "${path.module}/../../../../deploy/helm"

  image_registry = "ghcr.io/drewjocham"
  image_tag      = "v1.4.2"

  backend_enabled    = true
  frontend_enabled   = true
  monitoring_enabled = true

  argocd_url             = "https://argocd.example.com"
  deepseek_api_key       = var.deepseek_api_key
  argocd_token           = var.argocd_token
  grafana_admin_password = var.grafana_admin_password
}
```

## Providers

The caller wires Helm + Kubernetes providers — usually against an EKS
cluster:

```hcl
provider "kubernetes" {
  host                   = module.platform.cluster_endpoint
  cluster_ca_certificate = base64decode(module.platform.cluster_certificate_authority_data)
  token                  = data.aws_eks_cluster_auth.this.token
}

provider "helm" {
  kubernetes {
    host                   = module.platform.cluster_endpoint
    cluster_ca_certificate = base64decode(module.platform.cluster_certificate_authority_data)
    token                  = data.aws_eks_cluster_auth.this.token
  }
}
```

## Per-chart overrides

Need to tweak something the module doesn't expose? Use the
`extra_values_*` map; it merges over the module's computed defaults
without forking the chart values.
