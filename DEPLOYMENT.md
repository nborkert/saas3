# ComplianceSync API - Deployment Guide

This guide walks through deploying the ComplianceSync API to Google Cloud Platform.

## Prerequisites

- GCP project created
- `gcloud` CLI installed and configured
- Docker installed (for local testing)
- Firebase project linked to your GCP project

## Step 1: Enable Required APIs

```bash
# Set your project ID
export PROJECT_ID=your-gcp-project-id
gcloud config set project $PROJECT_ID

# Enable required APIs
gcloud services enable \
    firestore.googleapis.com \
    storage.googleapis.com \
    identitytoolkit.googleapis.com \
    run.googleapis.com \
    cloudbuild.googleapis.com \
    secretmanager.googleapis.com
```

## Step 2: Create Firestore Database

```bash
# Create Firestore database in Native mode
gcloud firestore databases create \
    --location=us-central1 \
    --type=firestore-native
```

## Step 3: Create Cloud Storage Bucket

```bash
# Create bucket for evidence files
export BUCKET_NAME=${PROJECT_ID}-compliancesync-evidence
gcloud storage buckets create gs://${BUCKET_NAME} \
    --location=us-central1 \
    --uniform-bucket-level-access

# Enable versioning for compliance
gcloud storage buckets update gs://${BUCKET_NAME} --versioning
```

## Step 4: Create Service Account

```bash
# Create service account
gcloud iam service-accounts create compliancesync-api \
    --display-name="ComplianceSync API Service Account"

export SERVICE_ACCOUNT=compliancesync-api@${PROJECT_ID}.iam.gserviceaccount.com

# Grant necessary permissions
gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/datastore.user"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/storage.objectAdmin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/secretmanager.secretAccessor"
```

## Step 5: Store Secrets in Secret Manager

```bash
# Stripe secret key
echo -n "your-stripe-secret-key" | gcloud secrets create stripe-secret-key --data-file=-

# SendGrid API key
echo -n "your-sendgrid-api-key" | gcloud secrets create sendgrid-api-key --data-file=-

# Grant service account access to secrets
gcloud secrets add-iam-policy-binding stripe-secret-key \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding sendgrid-api-key \
    --member="serviceAccount:${SERVICE_ACCOUNT}" \
    --role="roles/secretmanager.secretAccessor"
```

## Step 6: Configure Firebase Identity Platform

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project
3. Navigate to **Authentication** → **Sign-in method**
4. Enable **Email/Password** authentication
5. (Optional) Enable **Google** and **Microsoft** OAuth providers

## Step 7: Build and Deploy to Cloud Run

```bash
# Build and push container image
gcloud builds submit --tag gcr.io/${PROJECT_ID}/compliancesync-api

# Deploy to Cloud Run
gcloud run deploy compliancesync-api \
    --image gcr.io/${PROJECT_ID}/compliancesync-api \
    --platform managed \
    --region us-central1 \
    --allow-unauthenticated \
    --service-account ${SERVICE_ACCOUNT} \
    --set-env-vars "GCP_PROJECT_ID=${PROJECT_ID},STORAGE_BUCKET=${BUCKET_NAME},ENVIRONMENT=production" \
    --set-secrets "STRIPE_SECRET_KEY=stripe-secret-key:latest,SENDGRID_API_KEY=sendgrid-api-key:latest" \
    --max-instances 10 \
    --min-instances 0 \
    --memory 512Mi \
    --timeout 60s \
    --concurrency 80
```

## Step 8: Get Service URL

```bash
# Get the Cloud Run service URL
gcloud run services describe compliancesync-api \
    --region us-central1 \
    --format 'value(status.url)'
```

## Step 9: Test the Deployment

```bash
# Get the service URL
export API_URL=$(gcloud run services describe compliancesync-api --region us-central1 --format 'value(status.url)')

# Test health endpoint
curl ${API_URL}/health

# Expected response:
# {
#   "status": "healthy",
#   "service": "compliancesync-api"
# }
```

## Step 10: Set Up Monitoring (Recommended)

### Create Uptime Check

```bash
gcloud monitoring uptime create \
    --display-name="ComplianceSync API Health Check" \
    --resource-type=uptime-url \
    --resource-labels=host=${API_URL#https://}
```

### Create Alerting Policy

1. Go to **Cloud Console** → **Monitoring** → **Alerting**
2. Create new alerting policy for:
   - Error rate > 1%
   - 95th percentile latency > 2s
   - Instance count = 0 for > 5 minutes

## Step 11: Configure Custom Domain (Optional)

```bash
# Map custom domain to Cloud Run service
gcloud run domain-mappings create \
    --service compliancesync-api \
    --domain api.yourdomain.com \
    --region us-central1
```

Follow the instructions to configure DNS records.

## Step 12: Set Up CI/CD (Recommended)

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy to Cloud Run

on:
  push:
    branches:
      - main

jobs:
  deploy:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - id: auth
        uses: google-github-actions/auth@v1
        with:
          credentials_json: ${{ secrets.GCP_SA_KEY }}

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v1

      - name: Build and Deploy
        run: |
          gcloud builds submit --tag gcr.io/${{ secrets.PROJECT_ID }}/compliancesync-api
          gcloud run deploy compliancesync-api \
            --image gcr.io/${{ secrets.PROJECT_ID }}/compliancesync-api \
            --region us-central1 \
            --platform managed
```

## Troubleshooting

### "Permission denied" errors

Verify service account has required roles:

```bash
gcloud projects get-iam-policy $PROJECT_ID \
    --flatten="bindings[].members" \
    --filter="bindings.members:serviceAccount:${SERVICE_ACCOUNT}"
```

### Firestore connection issues

Ensure Firestore API is enabled and database is created:

```bash
gcloud services list --enabled | grep firestore
gcloud firestore databases list
```

### Cloud Storage signed URL errors

Verify service account has storage admin permissions:

```bash
gcloud storage buckets get-iam-policy gs://${BUCKET_NAME}
```

## Rollback Procedure

To rollback to a previous revision:

```bash
# List revisions
gcloud run revisions list --service compliancesync-api --region us-central1

# Update traffic to previous revision
gcloud run services update-traffic compliancesync-api \
    --region us-central1 \
    --to-revisions REVISION-NAME=100
```

## Cost Optimization

### Development Environment

For development, use lower limits:

```bash
gcloud run deploy compliancesync-api \
    --max-instances 2 \
    --min-instances 0 \
    --memory 256Mi \
    --concurrency 40
```

### Production Environment

For production with higher traffic:

```bash
gcloud run deploy compliancesync-api \
    --max-instances 50 \
    --min-instances 1 \
    --memory 1Gi \
    --concurrency 100
```

## Security Hardening

### Enable Binary Authorization (Recommended for Production)

```bash
gcloud beta run services update compliancesync-api \
    --region us-central1 \
    --binary-authorization default
```

### Configure VPC Connector (Optional)

If you need to access resources in a VPC:

```bash
# Create VPC connector
gcloud compute networks vpc-access connectors create compliancesync-connector \
    --region us-central1 \
    --range 10.8.0.0/28

# Update Cloud Run service
gcloud run services update compliancesync-api \
    --region us-central1 \
    --vpc-connector compliancesync-connector
```

## Maintenance

### Update Dependencies

```bash
# Update Go dependencies
go get -u ./...
go mod tidy

# Rebuild and redeploy
gcloud builds submit --tag gcr.io/${PROJECT_ID}/compliancesync-api
```

### View Logs

```bash
# Stream logs
gcloud run services logs tail compliancesync-api --region us-central1

# View recent logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=compliancesync-api" \
    --limit 100 \
    --format json
```

## Next Steps

1. **Seed Requirement Templates**: Create regulatory requirement templates in Firestore
2. **Configure Pub/Sub Workers**: Set up background job processing
3. **Set Up Stripe Webhooks**: Configure webhook endpoint in Stripe dashboard
4. **Configure SendGrid**: Set up email templates
5. **Load Testing**: Run load tests to optimize instance settings
6. **Disaster Recovery**: Set up automated Firestore backups

## Support

For issues or questions, refer to:
- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [Firestore Documentation](https://cloud.google.com/firestore/docs)
- [Firebase Authentication Documentation](https://firebase.google.com/docs/auth)
