#!/bin/bash

# ComplianceSync - Rollback Script
# Rollback Cloud Run service to a previous revision

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo "========================================="
echo "ComplianceSync Rollback"
echo "========================================="
echo ""

# Select environment
echo "Select environment:"
echo "1) Development"
echo "2) Staging"
echo "3) Production"
read -p "Enter choice [1-3]: " env_choice

case $env_choice in
    1)
        ENVIRONMENT="development"
        SERVICE_NAME="compliancesync-api-dev"
        ;;
    2)
        ENVIRONMENT="staging"
        SERVICE_NAME="compliancesync-api-staging"
        ;;
    3)
        ENVIRONMENT="production"
        SERVICE_NAME="compliancesync-api"
        log_warn "You are about to rollback PRODUCTION!"
        ;;
    *)
        log_error "Invalid choice"
        exit 1
        ;;
esac

read -p "Enter GCP Project ID: " PROJECT_ID
read -p "Enter region [us-central1]: " REGION
REGION=${REGION:-us-central1}

# Set project
gcloud config set project "$PROJECT_ID"

# List revisions
log_info "Fetching available revisions for $SERVICE_NAME..."
echo ""

gcloud run revisions list \
    --service="$SERVICE_NAME" \
    --region="$REGION" \
    --format="table(metadata.name,status.conditions[0].status,spec.containers[0].image,metadata.creationTimestamp)" \
    --sort-by="~metadata.creationTimestamp" \
    --limit=10

echo ""
read -p "Enter revision name to rollback to: " REVISION_NAME

if [ -z "$REVISION_NAME" ]; then
    log_error "Revision name cannot be empty"
    exit 1
fi

# Verify revision exists
if ! gcloud run revisions describe "$REVISION_NAME" \
    --region="$REGION" \
    --platform=managed &> /dev/null; then
    log_error "Revision $REVISION_NAME not found"
    exit 1
fi

echo ""
log_warn "This will route 100% of traffic to revision: $REVISION_NAME"
read -p "Are you sure you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    log_info "Rollback cancelled"
    exit 0
fi

# Perform rollback
log_info "Rolling back to $REVISION_NAME..."

gcloud run services update-traffic "$SERVICE_NAME" \
    --region="$REGION" \
    --to-revisions="$REVISION_NAME=100"

# Get service URL
SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" \
    --region="$REGION" \
    --format='value(status.url)')

log_info "Rollback complete!"
echo ""
echo "Service URL: $SERVICE_URL"
echo ""

# Test health endpoint
log_info "Testing health endpoint..."
sleep 5
HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${SERVICE_URL}/health")

if [ "$HEALTH_STATUS" -eq 200 ]; then
    log_info "Health check passed!"
else
    log_error "Health check failed with status: $HEALTH_STATUS"
    log_error "You may need to rollback to a different revision"
fi

echo ""
log_info "Current traffic split:"
gcloud run services describe "$SERVICE_NAME" \
    --region="$REGION" \
    --format="table(status.traffic[].revisionName,status.traffic[].percent)"
