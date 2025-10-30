terraform {
  required_version = ">= 1.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }

  # Uncomment to use GCS backend for state management
  # backend "gcs" {
  #   bucket = "your-terraform-state-bucket"
  #   prefix = "terraform/state"
  # }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Enable required APIs
resource "google_project_service" "services" {
  for_each = toset([
    "firestore.googleapis.com",
    "storage.googleapis.com",
    "identitytoolkit.googleapis.com",
    "run.googleapis.com",
    "cloudbuild.googleapis.com",
    "artifactregistry.googleapis.com",
    "secretmanager.googleapis.com",
    "cloudscheduler.googleapis.com",
    "pubsub.googleapis.com",
    "logging.googleapis.com",
    "monitoring.googleapis.com",
    "iamcredentials.googleapis.com",
  ])

  service            = each.value
  disable_on_destroy = false
}

# Firestore Database (must be created manually or via gcloud)
# Note: Terraform doesn't fully support Firestore database creation yet
# Use: gcloud firestore databases create --location=us-central1 --type=firestore-native

# Cloud Storage bucket for evidence files
resource "google_storage_bucket" "evidence_bucket" {
  name                        = "${var.project_id}-compliancesync-evidence-${var.environment}"
  location                    = var.region
  uniform_bucket_level_access = true
  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      num_newer_versions = 3
      days_since_noncurrent_time = 90
    }
  }

  depends_on = [google_project_service.services]
}

# Artifact Registry repository
resource "google_artifact_registry_repository" "container_repo" {
  location      = var.region
  repository_id = "compliancesync-${var.environment}"
  description   = "ComplianceSync container images for ${var.environment}"
  format        = "DOCKER"

  depends_on = [google_project_service.services]
}

# Service Account for Cloud Run
resource "google_service_account" "api_service_account" {
  account_id   = "compliancesync-${var.environment}"
  display_name = "ComplianceSync API Service Account (${var.environment})"
}

# IAM roles for service account
resource "google_project_iam_member" "service_account_roles" {
  for_each = toset([
    "roles/datastore.user",
    "roles/storage.objectAdmin",
    "roles/secretmanager.secretAccessor",
    "roles/pubsub.publisher",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/cloudtrace.agent",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.api_service_account.email}"
}

# Secret Manager secrets
resource "google_secret_manager_secret" "stripe_secret_key" {
  secret_id = "stripe-secret-key"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

resource "google_secret_manager_secret" "sendgrid_api_key" {
  secret_id = "sendgrid-api-key"

  replication {
    auto {}
  }

  depends_on = [google_project_service.services]
}

# Grant service account access to secrets
resource "google_secret_manager_secret_iam_member" "stripe_secret_access" {
  secret_id = google_secret_manager_secret.stripe_secret_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api_service_account.email}"
}

resource "google_secret_manager_secret_iam_member" "sendgrid_secret_access" {
  secret_id = google_secret_manager_secret.sendgrid_api_key.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.api_service_account.email}"
}

# Pub/Sub topics for background jobs
resource "google_pubsub_topic" "evidence_processing" {
  name = "evidence-processing"

  depends_on = [google_project_service.services]
}

resource "google_pubsub_topic" "report_generation" {
  name = "report-generation"

  depends_on = [google_project_service.services]
}

resource "google_pubsub_topic" "integration_sync" {
  name = "integration-sync"

  depends_on = [google_project_service.services]
}

# Pub/Sub subscriptions
resource "google_pubsub_subscription" "evidence_processing_sub" {
  name  = "evidence-processing-sub"
  topic = google_pubsub_topic.evidence_processing.name

  ack_deadline_seconds       = 60
  message_retention_duration = "604800s" # 7 days

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }
}

resource "google_pubsub_subscription" "report_generation_sub" {
  name  = "report-generation-sub"
  topic = google_pubsub_topic.report_generation.name

  ack_deadline_seconds       = 60
  message_retention_duration = "604800s"

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }
}

resource "google_pubsub_subscription" "integration_sync_sub" {
  name  = "integration-sync-sub"
  topic = google_pubsub_topic.integration_sync.name

  ack_deadline_seconds       = 60
  message_retention_duration = "604800s"

  retry_policy {
    minimum_backoff = "10s"
    maximum_backoff = "600s"
  }
}

# Workload Identity Pool for GitHub Actions
resource "google_iam_workload_identity_pool" "github_pool" {
  workload_identity_pool_id = "github-pool"
  display_name              = "GitHub Actions Pool"
  description               = "Workload Identity Pool for GitHub Actions CI/CD"
}

# Workload Identity Provider
resource "google_iam_workload_identity_pool_provider" "github_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                       = "GitHub Provider"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }

  attribute_condition = "assertion.repository=='${var.github_repository}'"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

# Grant Workload Identity User role to service account
resource "google_service_account_iam_member" "workload_identity_user" {
  service_account_id = google_service_account.api_service_account.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/${var.github_repository}"
}

# Grant additional permissions for CI/CD service account
resource "google_project_iam_member" "cicd_roles" {
  for_each = toset([
    "roles/artifactregistry.writer",
    "roles/run.admin",
    "roles/iam.serviceAccountUser",
  ])

  project = var.project_id
  role    = each.value
  member  = "serviceAccount:${google_service_account.api_service_account.email}"
}
