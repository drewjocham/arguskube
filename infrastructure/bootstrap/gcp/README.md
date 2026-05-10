# `bootstrap/gcp`

Creates the GCS bucket every `live/<env>/gcp-*` workspace uses for
remote state.

```sh
cd infrastructure/bootstrap/gcp
terraform init
terraform apply -var project_id=<your-gcp-project>
```

Versioning is on; old object versions are pruned after 10 newer
versions exist (so `apply` history doesn't grow forever, but recent
rollbacks remain possible).
