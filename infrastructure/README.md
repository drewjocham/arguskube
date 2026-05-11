# `infrastructure/`

Terraform for Argus, organised the way a real SRE team would
keep it: reusable modules in one place, per-environment compositions
in another, bootstrap state isolated from everything else.

## Layout

```
infrastructure/
├── bootstrap/          # one-time per cloud account: state bucket + lock table
│   ├── aws/            # creates s3://argus-tfstate + dynamodb lock
│   └── gcp/            # creates gs://argus-tfstate
│
├── modules/            # reusable, environment-agnostic building blocks
│   ├── eks-platform/        # AWS: VPC + EKS + node group + scheduled scaling
│   ├── argus-helm/    # Helm releases for every Argus component
│   ├── flink-vm/            # GCP: Compute Engine VM running Apache Flink
│   └── cloud-run/           # GCP: Cloud Run services (backend, frontend, mcp)
│
└── live/               # per-environment compositions (root modules)
    ├── dev/
    │   ├── aws/             # EKS dev cluster + Argus releases
    │   ├── gcp-flink-vm/    # Flink anomaly-detection VM
    │   └── gcp-cloud-run/   # Cloud Run dev deploy
    ├── staging/
    │   └── aws/
    └── prod/
        └── aws/
```

### Why this shape?

- **Modules are pure**: no environment names, no hard-coded sizes, no
  secrets. They take inputs and return outputs. You can drop
  `eks-platform` into any other repo unchanged.

- **Live workspaces are thin**: each `live/<env>/<stack>` is just
  `backend.tf` + `providers.tf` + `main.tf` calling modules with
  environment-specific values. The diff between dev and prod is small
  and obvious — exactly what an SRE wants when paged at 3am.

- **One state per (env, stack)**: blowing up dev can't damage prod.
  Each `live/<env>/<stack>` has its own S3 key (or GCS prefix), so
  `terraform apply` in one workspace cannot lock or mutate another's
  state.

## First-time setup

### AWS

```sh
# 1. Bootstrap the state bucket + lock table (once per AWS account)
cd infrastructure/bootstrap/aws
terraform init
terraform apply

# 2. Deploy the dev cluster
cd ../../live/dev/aws
cp terraform.tfvars.example terraform.tfvars   # fill in secrets
terraform init
terraform apply
```

### GCP

```sh
cd infrastructure/bootstrap/gcp
terraform init
terraform apply -var project_id=my-gcp-project

cd ../../live/dev/gcp-flink-vm
terraform init
terraform apply -var project_id=my-gcp-project ...
```

## Typical workflows

| Goal                                       | Where to look                                       |
| ------------------------------------------ | --------------------------------------------------- |
| Add a new Helm chart to all environments   | `modules/argus-helm/main.tf` + new variable   |
| Bump the EKS Kubernetes version            | `live/<env>/aws/main.tf` → `cluster_version`        |
| Resize the dev node group                  | `live/dev/aws/main.tf` → `node_group_*`             |
| Switch to a private cluster API in staging | `live/staging/aws/main.tf` → `cluster_endpoint_public_access = false` |
| Add a new environment                      | `cp -r live/staging live/qa`, edit `locals` + `backend.tf` |
| Add a new cloud (Azure, …)                 | New `modules/<thing>` + new `live/<env>/azure-*`    |

## Migrating from the old layout

The previous layout was:

- `deploy/terraform/` — one big root module mixing AWS infra + Helm
- `infra/terraform/{flink-vm,cloud-run}/` — standalone modules

If you have running state pointing at `deploy/terraform/`, migrate it:

```sh
# Set up backend + workspace
cd infrastructure/live/dev/aws
terraform init

# Move each existing resource into the new module addresses
terraform state mv -state=../../../../deploy/terraform/terraform.tfstate \
  module.vpc module.platform.module.vpc
terraform state mv -state=../../../../deploy/terraform/terraform.tfstate \
  module.eks module.platform.module.eks
terraform state mv -state=../../../../deploy/terraform/terraform.tfstate \
  aws_security_group.cluster module.platform.aws_security_group.cluster
terraform state mv -state=../../../../deploy/terraform/terraform.tfstate \
  helm_release.backend module.apps.helm_release.backend[0]
# … repeat for each helm_release the old layout had …
```

Run `terraform plan` after each `mv` and verify it's a no-op before
the next one.

If you've never run `terraform apply` from the old paths, just delete
them — there's no state to preserve.

## Conventions

- Provider versions are pinned in every module's `versions.tf` and in
  every live workspace's `providers.tf`. Update both when bumping.
- Modules never declare provider configurations — only `required_providers`
  blocks. Root modules supply the provider config. This keeps modules
  reusable across multiple aliases (e.g. multi-region deploys).
- Resource names always include `var.environment` or a name prefix —
  no two environments share AWS/GCP resource names by accident.
- Secrets enter via `*.tfvars` (gitignored) or `TF_VAR_*` env vars.
  Never check secrets into source control.
