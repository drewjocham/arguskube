# `cloud-run`

GCP module: every KubeWatcher service deployed as a Cloud Run revision
(backend, frontend, mcp), plus the Secret Manager containers and IAM
bindings they need.

Files:

- `main.tf` — the three `google_cloud_run_service` resources
- `secrets.tf` — Secret Manager containers (values populated out-of-band)
- `iam.tf` — invoker bindings (frontend public, backend/mcp service-only)

The module declares no provider — wire `provider "google"` in the
caller (root module).

## Secret population

This module creates the secret *containers* but never the *versions*.
After `terraform apply`, populate the values yourself:

```sh
echo -n "$DEEPSEEK_KEY" | gcloud secrets versions add kubewatcher-deepseek-api-key --data-file=-
echo -n "$FLINK_KEY"    | gcloud secrets versions add kubewatcher-flink-api-key    --data-file=-
```

That keeps secret values out of state and out of plan output.
