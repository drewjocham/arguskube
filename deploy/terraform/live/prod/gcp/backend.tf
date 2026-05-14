terraform {
  backend "gcs" {
    bucket = "kubewatcher-tfstate"
    prefix = "live/prod/gcp"
  }
}
