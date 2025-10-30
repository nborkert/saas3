#!/bin/bash

# ComplianceSync - Setup Validation Script
# Validates that all required GCP resources and configurations are in place

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_check() {
    echo -e "${BLUE}[→]${NC} $1"
}

ERRORS=0
WARNINGS=0

echo "========================================="
echo "ComplianceSync Setup Validation"
echo "========================================="
echo ""

read -p "Enter GCP Project ID: " PROJECT_ID
read -p "Enter environment [development]: " ENVIRONMENT
ENVIRONMENT=${ENVIRONMENT:-development}

case $ENVIRONMENT in
    production)
        SERVICE_SUFFIX=""
        ARTIFACT_REPO="compliancesync-prod"
        ;;
    staging)
        SERVICE_SUFFIX="-staging"
        ARTIFACT_REPO="compliancesync-staging"
        ;;
    development)
        SERVICE_SUFFIX="-dev"
        ARTIFACT_REPO="compliancesync-dev"
        ;;
    *)
        log_error "Invalid environment"
        exit 1
        ;;
esac

SERVICE_NAME="compliancesync-api${SERVICE_SUFFIX}"
SERVICE_ACCOUNT="compliancesync-${ENVIRONMENT}@${PROJECT_ID}.iam.gserviceaccount.com"
BUCKET_NAME="${PROJECT_ID}-compliancesync-evidence-${ENVIRONMENT}"
REGION="us-central1"

echo "Validating configuration for:"
echo "  Project: $PROJECT_ID"
echo "  Environment: $ENVIRONMENT"
echo ""

# Set project
gcloud config set project "$PROJECT_ID" --quiet

# Check APIs
echo "Checking required APIs..."
REQUIRED_APIS=(
    "firestore.googleapis.com"
    "storage.googleapis.com"
    "identitytoolkit.googleapis.com"
    "run.googleapis.com"
    "cloudbuild.googleapis.com"
    "artifactregistry.googleapis.com"
    "secretmanager.googleapis.com"
    "pubsub.googleapis.com"
)

for api in "${REQUIRED_APIS[@]}"; do
    log_check "Checking $api..."
    if gcloud services list --enabled --filter="name:$api" --format="value(name)" | grep -q "$api"; then
        log_info "$api is enabled"
    else
        log_error "$api is NOT enabled"
        ((ERRORS++))
    fi
done

echo ""
echo "Checking Firestore..."
log_check "Checking Firestore database..."
if gcloud firestore databases list --format="value(name)" 2>&1 | grep -q "(default)"; then
    log_info "Firestore database exists"
else
    log_error "Firestore database does NOT exist"
    echo "  Run: gcloud firestore databases create --location=us-central1 --type=firestore-native"
    ((ERRORS++))
fi

echo ""
echo "Checking Cloud Storage..."
log_check "Checking bucket gs://${BUCKET_NAME}..."
if gsutil ls -p "$PROJECT_ID" 2>/dev/null | grep -q "gs://${BUCKET_NAME}/"; then
    log_info "Storage bucket exists"

    # Check versioning
    if gsutil versioning get "gs://${BUCKET_NAME}" | grep -q "Enabled"; then
        log_info "Versioning is enabled"
    else
        log_warn "Versioning is NOT enabled"
        ((WARNINGS++))
    fi
else
    log_error "Storage bucket does NOT exist"
    ((ERRORS++))
fi

echo ""
echo "Checking Artifact Registry..."
log_check "Checking repository ${ARTIFACT_REPO}..."
if gcloud artifacts repositories describe "$ARTIFACT_REPO" \
    --location="$REGION" --format="value(name)" 2>/dev/null | grep -q "$ARTIFACT_REPO"; then
    log_info "Artifact Registry repository exists"
else
    log_error "Artifact Registry repository does NOT exist"
    ((ERRORS++))
fi

echo ""
echo "Checking Service Account..."
log_check "Checking service account ${SERVICE_ACCOUNT}..."
if gcloud iam service-accounts describe "$SERVICE_ACCOUNT" \
    --format="value(email)" 2>/dev/null | grep -q "$SERVICE_ACCOUNT"; then
    log_info "Service account exists"

    # Check IAM roles
    log_check "Checking IAM roles..."
    REQUIRED_ROLES=(
        "roles/datastore.user"
        "roles/storage.objectAdmin"
        "roles/secretmanager.secretAccessor"
    )

    for role in "${REQUIRED_ROLES[@]}"; do
        if gcloud projects get-iam-policy "$PROJECT_ID" \
            --flatten="bindings[].members" \
            --filter="bindings.role:$role AND bindings.members:serviceAccount:${SERVICE_ACCOUNT}" \
            --format="value(bindings.role)" 2>/dev/null | grep -q "$role"; then
            log_info "Has $role"
        else
            log_error "Missing $role"
            ((ERRORS++))
        fi
    done
else
    log_error "Service account does NOT exist"
    ((ERRORS++))
fi

echo ""
echo "Checking Secret Manager..."
SECRETS=("stripe-secret-key" "sendgrid-api-key")
for secret in "${SECRETS[@]}"; do
    log_check "Checking secret $secret..."
    if gcloud secrets describe "$secret" --format="value(name)" 2>/dev/null | grep -q "$secret"; then
        log_info "Secret $secret exists"

        # Check if it has versions
        if gcloud secrets versions list "$secret" --limit=1 --format="value(name)" 2>/dev/null | grep -q "1"; then
            log_info "Secret has versions"
        else
            log_warn "Secret exists but has NO versions"
            ((WARNINGS++))
        fi
    else
        log_warn "Secret $secret does NOT exist (optional)"
        ((WARNINGS++))
    fi
done

echo ""
echo "Checking Pub/Sub..."
TOPICS=("evidence-processing" "report-generation" "integration-sync")
for topic in "${TOPICS[@]}"; do
    log_check "Checking topic $topic..."
    if gcloud pubsub topics describe "$topic" --format="value(name)" 2>/dev/null | grep -q "$topic"; then
        log_info "Topic $topic exists"

        # Check subscription
        subscription="${topic}-sub"
        if gcloud pubsub subscriptions describe "$subscription" --format="value(name)" 2>/dev/null | grep -q "$subscription"; then
            log_info "Subscription $subscription exists"
        else
            log_warn "Subscription $subscription does NOT exist"
            ((WARNINGS++))
        fi
    else
        log_error "Topic $topic does NOT exist"
        ((ERRORS++))
    fi
done

echo ""
echo "Checking Workload Identity Federation..."
log_check "Checking workload identity pool..."
if gcloud iam workload-identity-pools describe "github-pool" \
    --location="global" --format="value(name)" 2>/dev/null | grep -q "github-pool"; then
    log_info "Workload Identity Pool exists"

    log_check "Checking workload identity provider..."
    if gcloud iam workload-identity-pools providers describe "github-provider" \
        --workload-identity-pool="github-pool" \
        --location="global" --format="value(name)" 2>/dev/null | grep -q "github-provider"; then
        log_info "Workload Identity Provider exists"
    else
        log_error "Workload Identity Provider does NOT exist"
        ((ERRORS++))
    fi
else
    log_error "Workload Identity Pool does NOT exist"
    ((ERRORS++))
fi

echo ""
echo "========================================="
echo "Validation Summary"
echo "========================================="
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    log_info "All checks passed! Your setup is complete."
    echo ""
    echo "Next steps:"
    echo "  1. Configure GitHub secrets (see CICD_GUIDE.md)"
    echo "  2. Push code to trigger deployment"
    echo "  3. Monitor deployment in GitHub Actions"
elif [ $ERRORS -eq 0 ]; then
    log_warn "Setup is mostly complete with $WARNINGS warning(s)"
    echo ""
    echo "Review the warnings above. The setup may still work,"
    echo "but some optional features might not be available."
else
    log_error "Setup is incomplete with $ERRORS error(s) and $WARNINGS warning(s)"
    echo ""
    echo "Please fix the errors above before deploying."
    echo "Run ./setup-gcp-project.sh to create missing resources."
    exit 1
fi

echo ""
echo "Useful commands:"
echo "  View logs: gcloud run services logs tail $SERVICE_NAME --region=$REGION"
echo "  Deploy manually: ./deploy-manual.sh"
echo "  View secrets: gcloud secrets list"
echo "  Test Firestore: gcloud firestore databases list"
