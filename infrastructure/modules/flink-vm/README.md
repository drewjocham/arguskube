# `flink-vm`

GCP module: a single Compute Engine VM running Apache Flink, plus the
firewalls and service account it needs to ingest from Kafka and emit
metrics. Used by the Argus anomaly-detection pipeline.

The module declares no provider — wire `provider "google"` in the
caller (root module).

## Usage

```hcl
module "flink" {
  source = "../../../modules/flink-vm"

  project_id = "argus-dev-12345"
  region     = "us-west1"
  zone       = "us-west1-a"

  network    = google_compute_network.argus.name
  subnetwork = google_compute_subnetwork.argus.name

  gateway_api_key = var.flink_gateway_api_key
  kafka_bootstrap_servers = "kafka.example.com:9092"
}
```

Outputs `gateway_url` (internal) and `service_account` for downstream
modules (e.g. `cloud-run`) that need to call into the Flink gateway.
