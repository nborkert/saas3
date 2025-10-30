#!/bin/bash

# ComplianceSync - Manual Deployment Script
# Use this script for manual deployments when you need to bypass GitHub Actions

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
if ! command -v gcloud &> /dev/null; then
    log_error "gcloud CLI is not installed"
    exit 1
fi

if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed"
    exit 1
fi

echo "========================================="
echo "ComplianceSync Manual Deployment"
echo "========================================="
echo ""

# Select environment
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

# Get configuration
read -p "Enter GCP Project ID: " PROJECT_ID
read -p "Enter region [us-central1]: " REGION
REGION=${REGION:-us-central1}

# Set environment-specific variables
case $ENVIRONMENT in
    production)
        SERVICE_NAME="compliancesync-api"
        MIN_INSTANCES=1
        MAX_INSTANCES=50
        MEMORY="1Gi"
        ARTIFACT_REPO="compliancesync-prod"
        ;;
    staging)
        SERVICE_NAME="compliancesync-api-staging"
        MIN_INSTANCES=0
        MAX_INSTANCES=10
        MEMORY="512Mi"
        ARTIFACT_REPO="compliancesync-staging"
        ;;
    development)
        SERVICE_NAME="compliancesync-api-dev"
        MIN_INSTANCES=0
        MAX_INSTANCES=5
        MEMORY="256Mi"
        ARTIFACT_REPO="compliancesync-dev"
        ;;
esac

SERVICE_ACCOUNT="compliancesync-${ENVIRONMENT}@${PROJECT_ID}.iam.gserviceaccount.com"
IMAGE_TAG=$(git rev-parse --short HEAD)
BUCKET_NAME="${PROJECT_ID}-compliancesync-evidence-${ENVIRONMENT}"

log_info "Configuration:"
echo "  Environment: $ENVIRONMENT"
echo "  Project: $PROJECT_ID"
echo "  Region: $REGION"
echo "  Service: $SERVICE_NAME"
echo "  Image Tag: $IMAGE_TAG"
echo ""

read -p "Continue with deployment? (y/n): " confirm
if [ "$confirm" != "y" ]; then
    exit 0
fi

# Set active project
log_info "Setting active project..."
gcloud config set project "$PROJECT_ID"

# Build image
log_info "Building Docker image..."
docker build -t "${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:${IMAGE_TAG}" .
docker tag "${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:${IMAGE_TAG}" \
    "${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:latest"

# Configure Docker for Artifact Registry
log_info "Configuring Docker authentication..."
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

# Push image
log_info "Pushing image to Artifact Registry..."
docker push "${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:${IMAGE_TAG}"
docker push "${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:latest"

# Deploy to Cloud Run
log_info "Deploying to Cloud Run..."
gcloud run deploy "$SERVICE_NAME" \
    --image="${REGION}-docker.pkg.dev/${PROJECT_ID}/${ARTIFACT_REPO}/compliancesync-api:${IMAGE_TAG}" \
    --platform=managed \
    --region="$REGION" \
    --service-account="$SERVICE_ACCOUNT" \
    --allow-unauthenticated \
    --min-instances="$MIN_INSTANCES" \
    --max-instances="$MAX_INSTANCES" \
    --memory="$MEMORY" \
    --cpu=1 \
    --timeout=60s \
    --concurrency=80 \
    --set-env-vars="GCP_PROJECT_ID=${PROJECT_ID},STORAGE_BUCKET=${BUCKET_NAME},ENVIRONMENT=${ENVIRONMENT}" \
    --set-secrets="STRIPE_SECRET_KEY=stripe-secret-key:latest,SENDGRID_API_KEY=sendgrid-api-key:latest"

# Get service URL
SERVICE_URL=$(gcloud run services describe "$SERVICE_NAME" \
    --region="$REGION" \
    --format='value(status.url)')

log_info "Deployment complete!"
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
    exit 1
fi

echo ""
log_info "Deployment successful!"
echo "Test the API: curl ${SERVICE_URL}/health"
