terraform {
  backend "s3" {
    bucket         = "argus-tfstate"
    key            = "live/prod/aws/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    kms_key_id     = "alias/aws/s3"
    dynamodb_table = "argus-tfstate-lock"
  }
}
