# `bootstrap/aws`

One-time setup for the shared S3 + DynamoDB used by every
`live/<env>/aws` workspace's remote-state backend.

## When to run

- First time you deploy Argus into a new AWS account.
- Never again, unless you're migrating accounts.

## Run

```sh
cd infrastructure/bootstrap/aws
terraform init
terraform apply
```

This creates:

- `s3://argus-tfstate` (versioning + KMS-SSE + public access blocked)
- `dynamodb table argus-tfstate-lock` (PITR + encryption)

## After apply

The resulting `terraform.tfstate` is *itself* not in S3 — it's local.
Two options for safekeeping:

1. **Migrate it into the bucket it just created.** Add a backend block
   pointing at a separate `bootstrap/` key, then `terraform init -migrate-state`.
2. **Commit the file to a secrets vault** (1Password, Vault, …) outside
   git. The whole module is ~5 resources — easy to re-import if lost.

Pick (1) unless your org policy forbids mutually-recursive bootstrap.
