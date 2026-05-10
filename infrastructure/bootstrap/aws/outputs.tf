output "state_bucket" {
  description = "Use this in every live/<env>/aws/backend.tf as `bucket`."
  value       = aws_s3_bucket.tfstate.id
}

output "lock_table" {
  description = "Use this in every live/<env>/aws/backend.tf as `dynamodb_table`."
  value       = aws_dynamodb_table.tfstate_lock.id
}

output "region" {
  value = var.aws_region
}
