#!/bin/bash

# ComplianceSync - GCP Project Setup Script
# This script sets up the GCP infrastructure required for the ComplianceSync application
# Run this script once per environment (dev, staging, production)

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are installed
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v gcloud &> /dev/null; then
        log_error "gcloud CLI is not installed. Please install it from https://cloud.google.com/sdk/docs/install"
        exit 1
    fi

    if ! command -v jq &> /dev/null; then
        log_warn "jq is not installed. Some features may not work properly."
    fi

    log_info "Prerequisites check completed."
}

# Prompt for environment configuration
prompt_environment() {
    echo ""
    echo "========================================="
    echo "ComplianceSync GCP Project Setup"
    echo "========================================="
    echo ""

    # Environment selection
    echo "Select environment:"
    echo "1) Development"
    echo "2) Staging"
    echo "3) Production"
    read -p "Enter choice [1-3]: " env_choice

    case $env_choice in
        1) ENVIRONMENT="development" ;;
        2) ENVIRONMENT="staging" ;;
        3) ENVIRONMENT="production" ;;
        *) log_error "Invalid choice"; exit 1 ;;
    esac

    # Project ID
    read -p "Enter GCP Project ID: " PROJECT_ID

    # Region
    read -p "Enter GCP region [us-central1]: " REGION
    REGION=${REGION:-us-central1}

    # Service Account Name
    SERVICE_ACCOUNT_NAME="compliancesync-${ENVIRONMENT}"
    SERVICE_ACCOUNT="${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

    # Bucket Name
    BUCKET_NAME="${PROJECT_ID}-compliancesync-evidence-${ENVIRONMENT}"

    # Artifact Registry Repository
    ARTIFACT_REPO="compliancesync-${ENVIRONMENT}"

    # Service Name
    if [ "$ENVIRONMENT" = "production" ]; then
        SERVICE_NAME="compliancesync-api"
    else
        SERVICE_NAME="compliancesync-api-${ENVIRONMENT}"
    fi

    echo ""
    log_info "Configuration:"
    echo "  Environment: $ENVIRONMENT"
    echo "  Project ID: $PROJECT_ID"
    echo "  Region: $REGION"
    echo "  Service Account: $SERVICE_ACCOUNT"
    echo "  Storage Bucket: $BUCKET_NAME"
    echo "  Artifact Registry: $ARTIFACT_REPO"
    echo "  Cloud Run Service: $SERVICE_NAME"
    echo ""

    read -p "Proceed with this configuration? (y/n): " confirm
    if [ "$confirm" != "y" ]; then
        log_error "Setup cancelled."
        exit 1
    fi
}

# Set the active project
set_project() {
    log_info "Setting active GCP project to $PROJECT_ID..."
    gcloud config set project "$PROJECT_ID"
}

# Enable required APIs
enable_apis() {
    log_info "Enabling required GCP APIs..."

    REQUIRED_APIS=(
        "firestore.googleapis.com"
        "storage.googleapis.com"
        "identitytoolkit.googleapis.com"
        "run.googleapis.com"
        "cloudbuild.googleapis.com"
        "artifactregistry.googleapis.com"
        "secretmanager.googleapis.com"
        "cloudscheduler.googleapis.com"
        "pubsub.googleapis.com"
        "logging.googleapis.com"
        "monitoring.googleapis.com"
        "iamcredentials.googleapis.com"
    )

    for api in "${REQUIRED_APIS[@]}"; do
        log_info "Enabling $api..."
        gcloud services enable "$api" --project="$PROJECT_ID"
    done

    log_info "All APIs enabled successfully."
}

# Create Firestore database
create_firestore() {
    log_info "Creating Firestore database..."

    # Check if database already exists
    if gcloud firestore databases list --project="$PROJECT_ID" 2>&1 | grep -q "(default)"; then
        log_warn "Firestore database already exists. Skipping creation."
    else
        gcloud firestore databases create \
            --location="$REGION" \
            --type=firestore-native \
            --project="$PROJECT_ID"
        log_info "Firestore database created."
    fi
}

# Create Cloud Storage bucket
create_storage_bucket() {
    log_info "Creating Cloud Storage bucket..."

    # Check if bucket already exists
    if gsutil ls -p "$PROJECT_ID" | grep -q "gs://${BUCKET_NAME}/"; then
        log_warn "Storage bucket already exists. Skipping creation."
    else
        gcloud storage buckets create "gs://${BUCKET_NAME}" \
            --location="$REGION" \
            --uniform-bucket-level-access \
            --project="$PROJECT_ID"

        # Enable versioning for compliance
        gcloud storage buckets update "gs://${BUCKET_NAME}" \
            --versioning \
            --project="$PROJECT_ID"

        # Set lifecycle rules to delete old versions after 90 days
        cat > /tmp/lifecycle.json <<EOF
{
  "lifecycle": {
    "rule": [
      {
        "action": {"type": "Delete"},
        "condition": {
          "numNewerVersions": 3,
          "daysSinceNoncurrentTime": 90
        }
      }
    ]
  }
}
EOF
        gsutil lifecycle set /tmp/lifecycle.json "gs://${BUCKET_NAME}"
        rm /tmp/lifecycle.json

        log_info "Storage bucket created with versioning enabled."
    fi
}

# Create Artifact Registry repository
create_artifact_registry() {
    log_info "Creating Artifact Registry repository..."

    # Check if repository already exists
    if gcloud artifacts repositories describe "$ARTIFACT_REPO" \
        --location="$REGION" \
        --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Artifact Registry repository already exists. Skipping creation."
    else
        gcloud artifacts repositories create "$ARTIFACT_REPO" \
            --repository-format=docker \
            --location="$REGION" \
            --description="ComplianceSync container images for $ENVIRONMENT" \
            --project="$PROJECT_ID"
        log_info "Artifact Registry repository created."
    fi
}

# Create service account
create_service_account() {
    log_info "Creating service account..."

    # Check if service account already exists
    if gcloud iam service-accounts describe "$SERVICE_ACCOUNT" \
        --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Service account already exists. Skipping creation."
    else
        gcloud iam service-accounts create "$SERVICE_ACCOUNT_NAME" \
            --display-name="ComplianceSync API Service Account ($ENVIRONMENT)" \
            --project="$PROJECT_ID"
        log_info "Service account created."
    fi

    # Grant necessary IAM roles
    log_info "Granting IAM roles to service account..."

    ROLES=(
        "roles/datastore.user"
        "roles/storage.objectAdmin"
        "roles/secretmanager.secretAccessor"
        "roles/pubsub.publisher"
        "roles/logging.logWriter"
        "roles/monitoring.metricWriter"
        "roles/cloudtrace.agent"
    )

    for role in "${ROLES[@]}"; do
        log_info "Granting $role..."
        gcloud projects add-iam-policy-binding "$PROJECT_ID" \
            --member="serviceAccount:${SERVICE_ACCOUNT}" \
            --role="$role" \
            --condition=None \
            --quiet
    done

    log_info "IAM roles granted successfully."
}

# Create secrets in Secret Manager
create_secrets() {
    log_info "Setting up Secret Manager..."

    # Stripe Secret Key
    if gcloud secrets describe "stripe-secret-key" --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Secret 'stripe-secret-key' already exists. Skipping creation."
    else
        read -sp "Enter Stripe Secret Key (or press Enter to skip): " stripe_key
        echo ""
        if [ -n "$stripe_key" ]; then
            echo -n "$stripe_key" | gcloud secrets create "stripe-secret-key" \
                --data-file=- \
                --replication-policy="automatic" \
                --project="$PROJECT_ID"
            log_info "Stripe secret created."
        else
            log_warn "Skipping Stripe secret creation. You can create it later."
        fi
    fi

    # SendGrid API Key
    if gcloud secrets describe "sendgrid-api-key" --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Secret 'sendgrid-api-key' already exists. Skipping creation."
    else
        read -sp "Enter SendGrid API Key (or press Enter to skip): " sendgrid_key
        echo ""
        if [ -n "$sendgrid_key" ]; then
            echo -n "$sendgrid_key" | gcloud secrets create "sendgrid-api-key" \
                --data-file=- \
                --replication-policy="automatic" \
                --project="$PROJECT_ID"
            log_info "SendGrid secret created."
        else
            log_warn "Skipping SendGrid secret creation. You can create it later."
        fi
    fi

    # Grant service account access to secrets
    log_info "Granting service account access to secrets..."

    for secret in "stripe-secret-key" "sendgrid-api-key"; do
        if gcloud secrets describe "$secret" --project="$PROJECT_ID" &> /dev/null; then
            gcloud secrets add-iam-policy-binding "$secret" \
                --member="serviceAccount:${SERVICE_ACCOUNT}" \
                --role="roles/secretmanager.secretAccessor" \
                --project="$PROJECT_ID" \
                --quiet
        fi
    done

    log_info "Secret Manager setup completed."
}

# Create Pub/Sub topics and subscriptions
create_pubsub() {
    log_info "Creating Pub/Sub topics and subscriptions..."

    # Topics for background jobs
    TOPICS=(
        "evidence-processing"
        "report-generation"
        "integration-sync"
    )

    for topic in "${TOPICS[@]}"; do
        if gcloud pubsub topics describe "$topic" --project="$PROJECT_ID" &> /dev/null; then
            log_warn "Topic '$topic' already exists. Skipping creation."
        else
            gcloud pubsub topics create "$topic" --project="$PROJECT_ID"
            log_info "Topic '$topic' created."
        fi

        # Create subscription for each topic
        subscription="${topic}-sub"
        if gcloud pubsub subscriptions describe "$subscription" --project="$PROJECT_ID" &> /dev/null; then
            log_warn "Subscription '$subscription' already exists. Skipping creation."
        else
            gcloud pubsub subscriptions create "$subscription" \
                --topic="$topic" \
                --ack-deadline=60 \
                --message-retention-duration=7d \
                --project="$PROJECT_ID"
            log_info "Subscription '$subscription' created."
        fi
    done

    log_info "Pub/Sub setup completed."
}

# Configure Workload Identity Federation for GitHub Actions
setup_workload_identity() {
    log_info "Setting up Workload Identity Federation for GitHub Actions..."

    read -p "Enter GitHub repository (format: owner/repo, e.g., nborkert/saas3): " GITHUB_REPO

    POOL_NAME="github-pool"
    POOL_ID="projects/$PROJECT_ID/locations/global/workloadIdentityPools/$POOL_NAME"
    PROVIDER_NAME="github-provider"

    # Create Workload Identity Pool
    if gcloud iam workload-identity-pools describe "$POOL_NAME" \
        --location="global" \
        --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Workload Identity Pool already exists. Skipping creation."
    else
        gcloud iam workload-identity-pools create "$POOL_NAME" \
            --location="global" \
            --display-name="GitHub Actions Pool" \
            --project="$PROJECT_ID"
        log_info "Workload Identity Pool created."
    fi

    # Create Workload Identity Provider
    if gcloud iam workload-identity-pools providers describe "$PROVIDER_NAME" \
        --workload-identity-pool="$POOL_NAME" \
        --location="global" \
        --project="$PROJECT_ID" &> /dev/null; then
        log_warn "Workload Identity Provider already exists. Skipping creation."
    else
        gcloud iam workload-identity-pools providers create-oidc "$PROVIDER_NAME" \
            --workload-identity-pool="$POOL_NAME" \
            --location="global" \
            --issuer-uri="https://token.actions.githubusercontent.com" \
            --attribute-mapping="google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository" \
            --attribute-condition="assertion.repository=='$GITHUB_REPO'" \
            --project="$PROJECT_ID"
        log_info "Workload Identity Provider created."
    fi

    # Grant Service Account Token Creator role
    gcloud iam service-accounts add-iam-policy-binding "$SERVICE_ACCOUNT" \
        --role="roles/iam.workloadIdentityUser" \
        --member="principalSet://iam.googleapis.com/${POOL_ID}/attribute.repository/${GITHUB_REPO}" \
        --project="$PROJECT_ID"

    # Display WIF configuration for GitHub Secrets
    WORKLOAD_IDENTITY_PROVIDER="projects/$(gcloud projects describe $PROJECT_ID --format='value(projectNumber)')/locations/global/workloadIdentityPools/$POOL_NAME/providers/$PROVIDER_NAME"

    echo ""
    log_info "Workload Identity Federation setup completed!"
    echo ""
    echo "========================================="
    echo "GitHub Secrets Configuration"
    echo "========================================="
    echo "Add the following secrets to your GitHub repository:"
    echo ""
    echo "Secret Name: GCP_PROJECT_ID_$(echo $ENVIRONMENT | tr '[:lower:]' '[:upper:]')"
    echo "Secret Value: $PROJECT_ID"
    echo ""
    echo "Secret Name: WIF_PROVIDER_$(echo $ENVIRONMENT | tr '[:lower:]' '[:upper:]')"
    echo "Secret Value: $WORKLOAD_IDENTITY_PROVIDER"
    echo ""
    echo "Secret Name: WIF_SERVICE_ACCOUNT_$(echo $ENVIRONMENT | tr '[:lower:]' '[:upper:]')"
    echo "Secret Value: $SERVICE_ACCOUNT"
    echo ""
    echo "Secret Name: STORAGE_BUCKET"
    echo "Secret Value: $BUCKET_NAME"
    echo "========================================="
    echo ""
}

# Summary
display_summary() {
    echo ""
    echo "========================================="
    echo "Setup Complete!"
    echo "========================================="
    echo ""
    log_info "The following resources have been created:"
    echo "  - Firestore database"
    echo "  - Cloud Storage bucket: gs://${BUCKET_NAME}"
    echo "  - Artifact Registry: ${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}"
    echo "  - Service Account: ${SERVICE_ACCOUNT}"
    echo "  - Secret Manager secrets (if provided)"
    echo "  - Pub/Sub topics and subscriptions"
    echo "  - Workload Identity Federation"
    echo ""
    log_info "Next steps:"
    echo "  1. Configure Firebase Identity Platform in the Firebase Console"
    echo "  2. Add the GitHub secrets shown above to your repository"
    echo "  3. Update secret values in Secret Manager if you skipped them"
    echo "  4. Run the deployment workflow from GitHub Actions"
    echo ""
    log_info "Useful commands:"
    echo "  View project: gcloud config get-value project"
    echo "  List secrets: gcloud secrets list --project=$PROJECT_ID"
    echo "  View logs: gcloud logging read --project=$PROJECT_ID --limit=50"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    prompt_environment
    set_project
    enable_apis
    create_firestore
    create_storage_bucket
    create_artifact_registry
    create_service_account
    create_secrets
    create_pubsub
    setup_workload_identity
    display_summary
}

# Run main function
main
