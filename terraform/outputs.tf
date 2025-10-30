output "service_account_email" {
  description = "Email of the service account"
  value       = google_service_account.api_service_account.email
}

output "storage_bucket_name" {
  description = "Name of the evidence storage bucket"
  value       = google_storage_bucket.evidence_bucket.name
}

output "artifact_registry_repository" {
  description = "Artifact Registry repository name"
  value       = google_artifact_registry_repository.container_repo.name
}

output "workload_identity_provider" {
  description = "Workload Identity Provider for GitHub Actions"
  value       = google_iam_workload_identity_pool_provider.github_provider.name
}

output "pubsub_topics" {
  description = "Pub/Sub topic names"
  value = {
    evidence_processing = google_pubsub_topic.evidence_processing.name
    report_generation   = google_pubsub_topic.report_generation.name
    integration_sync    = google_pubsub_topic.integration_sync.name
  }
}

output "github_secrets_configuration" {
  description = "GitHub Secrets that need to be configured"
  value = {
    GCP_PROJECT_ID        = var.project_id
    WIF_PROVIDER          = google_iam_workload_identity_pool_provider.github_provider.name
    WIF_SERVICE_ACCOUNT   = google_service_account.api_service_account.email
    STORAGE_BUCKET        = google_storage_bucket.evidence_bucket.name
  }
}
