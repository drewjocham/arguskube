# `bootstrap/gcp`

One-time setup for the shared GCS bucket used by every
`live/<env>/aws` workspace's remote-state backend.

## Prerequisites

- A GCP project with billing enabled
- `gcloud` CLI authenticated: `gcloud auth application-default login`

## When to run

- First time you deploy KubeWatcher into a new GCP project.
- Never again, unless you're migrating projects.

## Run

```sh
cd infra/terraform/bootstrap/gcp
gcloud auth application-default login
terraform init
terraform apply -var project_id=my-gcp-project
```

This creates:

- `gs://kubewatcher-tfstate` (versioning + uniform IAM + encryption)

## After apply

Migrate local state into the bucket:

```hcl
# uncomment in main.tf:
# terraform {
#   backend "gcs" {
#     bucket = "kubewatcher-tfstate"
#     prefix = "bootstrap/gcp"
#   }
# }
```

Then:

```sh
terraform init -migrate-state
```
