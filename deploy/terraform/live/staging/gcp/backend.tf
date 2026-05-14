terraform {
  backend "gcs" {
    bucket = "kubewatcher-tfstate"
    prefix = "live/staging/gcp"
  }
}
