#!/bin/bash

# ComplianceSync - View Logs Script
# Stream or view Cloud Run service logs

set -e

GREEN='\033[0;32m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo "========================================="
echo "ComplianceSync Logs Viewer"
echo "========================================="
echo ""

# Select environment
echo "Select environment:"
echo "1) Development"
echo "2) Staging"
echo "3) Production"
read -p "Enter choice [1-3]: " env_choice

case $env_choice in
    1) SERVICE_NAME="compliancesync-api-dev" ;;
    2) SERVICE_NAME="compliancesync-api-staging" ;;
    3) SERVICE_NAME="compliancesync-api" ;;
    *) echo "Invalid choice"; exit 1 ;;
esac

read -p "Enter GCP Project ID: " PROJECT_ID
read -p "Enter region [us-central1]: " REGION
REGION=${REGION:-us-central1}

# Set project
gcloud config set project "$PROJECT_ID"

# Select log mode
echo ""
echo "Select log mode:"
echo "1) Stream logs (real-time)"
echo "2) View recent logs (last 100 lines)"
echo "3) View logs with filters"
read -p "Enter choice [1-3]: " log_choice

case $log_choice in
    1)
        log_info "Streaming logs for $SERVICE_NAME..."
        gcloud run services logs tail "$SERVICE_NAME" --region="$REGION"
        ;;
    2)
        log_info "Fetching recent logs for $SERVICE_NAME..."
        gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=$SERVICE_NAME" \
            --limit=100 \
            --format="table(timestamp,severity,textPayload)" \
            --project="$PROJECT_ID"
        ;;
    3)
        echo ""
        echo "Filter options:"
        echo "1) Errors only"
        echo "2) Warnings and errors"
        echo "3) Custom severity"
        read -p "Enter choice [1-3]: " filter_choice

        case $filter_choice in
            1)
                SEVERITY_FILTER="severity>=ERROR"
                ;;
            2)
                SEVERITY_FILTER="severity>=WARNING"
                ;;
            3)
                read -p "Enter severity (DEBUG, INFO, WARNING, ERROR, CRITICAL): " CUSTOM_SEVERITY
                SEVERITY_FILTER="severity>=$CUSTOM_SEVERITY"
                ;;
        esac

        log_info "Fetching filtered logs..."
        gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=$SERVICE_NAME AND $SEVERITY_FILTER" \
            --limit=100 \
            --format="table(timestamp,severity,textPayload)" \
            --project="$PROJECT_ID"
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac
