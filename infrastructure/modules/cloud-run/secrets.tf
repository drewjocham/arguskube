# Secrets used by Cloud Run services.
#
# Secret VALUES are intentionally not stored in state — these resources
# create the *containers* in Secret Manager, then ops creates versions
# manually (or via a CI step) so production credentials never live in
# Terraform state. The Cloud Run service definitions reference the
# `latest` version of these secrets at deploy time.

resource "google_secret_manager_secret" "flink_api_key" {
  secret_id = "${var.name_prefix}-flink-api-key"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret" "deepseek_api_key" {
  secret_id = "${var.name_prefix}-deepseek-api-key"

  replication {
    auto {}
  }
}
