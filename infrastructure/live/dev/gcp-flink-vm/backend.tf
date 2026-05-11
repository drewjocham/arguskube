terraform {
  backend "gcs" {
    bucket = "argus-tfstate"
    prefix = "live/dev/gcp-flink-vm"
  }
}
