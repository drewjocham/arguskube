terraform {
  backend "gcs" {
    bucket = "kubewatcher-tfstate"
    prefix = "live/dev/gcp-cloud-run"
  }
}
