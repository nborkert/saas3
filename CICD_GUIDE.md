# ComplianceSync - CI/CD Deployment Guide

Complete guide for setting up and using the CI/CD pipeline for ComplianceSync.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Prerequisites](#prerequisites)
4. [Initial Setup](#initial-setup)
5. [GitHub Secrets Configuration](#github-secrets-configuration)
6. [Deployment Workflows](#deployment-workflows)
7. [Manual Deployment](#manual-deployment)
8. [Rollback Procedures](#rollback-procedures)
9. [Monitoring and Logs](#monitoring-and-logs)
10. [Troubleshooting](#troubleshooting)

---

## Overview

The ComplianceSync CI/CD pipeline automates the build, test, and deployment process for the Go-based REST API to Google Cloud Run. The pipeline uses GitHub Actions with Workload Identity Federation for secure, keyless authentication to GCP.

### Key Features

- **Automated Testing**: Runs Go tests on every push
- **Multi-Environment Support**: Development, Staging, and Production
- **Immutable Artifacts**: Container images tagged with Git commit SHA
- **Workload Identity Federation**: Secure authentication without service account keys
- **Automated Health Checks**: Verifies deployment success
- **Easy Rollback**: Quick rollback to previous revisions

---

## Architecture

```
GitHub Repository
    ↓
GitHub Actions Workflow
    ↓
    ├─→ Run Tests (Go test, vet)
    ↓
    ├─→ Build Docker Image
    ↓
    ├─→ Push to Artifact Registry
    ↓
    ├─→ Deploy to Cloud Run
    ↓
    └─→ Health Check
```

### Environments

| Environment | Branch | Min Instances | Max Instances | Memory |
|-------------|--------|---------------|---------------|--------|
| Development | Manual | 0 | 5 | 256Mi |
| Staging | Manual | 0 | 10 | 512Mi |
| Production | main | 1 | 50 | 1Gi |

---

## Prerequisites

Before setting up the CI/CD pipeline, ensure you have:

1. **GCP Account**: With billing enabled
2. **GitHub Repository**: Fork or clone of ComplianceSync
3. **gcloud CLI**: Installed and authenticated
4. **Terraform** (optional): For infrastructure as code
5. **Git**: Installed locally

---

## Initial Setup

### Option 1: Automated Setup (Recommended)

Use the provided setup script to automatically provision all GCP resources:

```bash
cd scripts
./setup-gcp-project.sh
```

This interactive script will:
- Prompt you to select an environment
- Enable all required GCP APIs
- Create Firestore database
- Create Cloud Storage bucket
- Create Artifact Registry repository
- Create and configure service account
- Set up Pub/Sub topics
- Configure Workload Identity Federation
- Display GitHub secrets for configuration

### Option 2: Terraform (Infrastructure as Code)

For a reproducible infrastructure setup:

```bash
cd terraform

# Initialize Terraform
terraform init

# Create variables file
cp terraform.tfvars.example terraform.tfvars

# Edit terraform.tfvars with your values
nano terraform.tfvars

# Review planned changes
terraform plan

# Apply configuration
terraform apply
```

**Note**: You still need to manually create the Firestore database:

```bash
gcloud firestore databases create \
  --location=us-central1 \
  --type=firestore-native
```

### Option 3: Manual Setup

Follow the detailed steps in [DEPLOYMENT.md](./DEPLOYMENT.md) for manual configuration.

---

## GitHub Secrets Configuration

After running the setup script or Terraform, configure the following GitHub repository secrets:

### Go to GitHub Repository Settings → Secrets and variables → Actions

#### Production Environment

| Secret Name | Description | Example Value |
|-------------|-------------|---------------|
| `GCP_PROJECT_ID_PROD` | Production GCP project ID | `compliancesync-prod-123456` |
| `WIF_PROVIDER_PROD` | Workload Identity Provider | `projects/123.../providers/github-provider` |
| `WIF_SERVICE_ACCOUNT_PROD` | Service account email | `compliancesync-production@...` |
| `STORAGE_BUCKET` | Cloud Storage bucket name | `compliancesync-prod-123456-evidence-production` |

#### Staging Environment (Optional)

| Secret Name | Description |
|-------------|-------------|
| `GCP_PROJECT_ID_STAGING` | Staging GCP project ID |
| `WIF_PROVIDER_STAGING` | Workload Identity Provider |
| `WIF_SERVICE_ACCOUNT_STAGING` | Service account email |

#### Development Environment (Optional)

| Secret Name | Description |
|-------------|-------------|
| `GCP_PROJECT_ID_DEV` | Development GCP project ID |
| `WIF_PROVIDER_DEV` | Workload Identity Provider |
| `WIF_SERVICE_ACCOUNT_DEV` | Service account email |

### How to Add Secrets

1. Navigate to your GitHub repository
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Enter the secret name and value
5. Click **Add secret**

---

## Deployment Workflows

### Automatic Deployment (Production)

The pipeline automatically deploys to production when code is pushed to the `main` branch:

```bash
git checkout main
git pull origin main
git add .
git commit -m "feat: add new feature"
git push origin main
```

GitHub Actions will:
1. Run tests
2. Build Docker image
3. Push to Artifact Registry
4. Deploy to Cloud Run
5. Run health checks
6. Tag the deployment

### Manual Deployment (Any Environment)

Trigger a manual deployment to any environment using GitHub Actions:

1. Go to your GitHub repository
2. Click **Actions** tab
3. Select **Deploy to Cloud Run** workflow
4. Click **Run workflow**
5. Select environment (development, staging, production)
6. Click **Run workflow**

### View Deployment Status

Monitor deployment progress:

1. Go to **Actions** tab in GitHub
2. Click on the running workflow
3. View real-time logs for each step
4. Check the deployment summary at the bottom

---

## Manual Deployment

For situations where you need to deploy manually (e.g., CI/CD is unavailable):

### Using the Deployment Script

```bash
cd scripts
./deploy-manual.sh
```

Follow the interactive prompts to:
- Select environment
- Enter project ID and region
- Build and push Docker image
- Deploy to Cloud Run
- Run health checks

### Using gcloud Directly

```bash
# Set variables
export PROJECT_ID=your-project-id
export REGION=us-central1
export SERVICE_NAME=compliancesync-api
export IMAGE_TAG=$(git rev-parse --short HEAD)

# Build and push
gcloud builds submit --tag ${REGION}-docker.pkg.dev/${PROJECT_ID}/compliancesync-prod/compliancesync-api:${IMAGE_TAG}

# Deploy
gcloud run deploy ${SERVICE_NAME} \
  --image ${REGION}-docker.pkg.dev/${PROJECT_ID}/compliancesync-prod/compliancesync-api:${IMAGE_TAG} \
  --region ${REGION} \
  --platform managed
```

---

## Rollback Procedures

### Quick Rollback

Use the rollback script for an interactive rollback:

```bash
cd scripts
./rollback.sh
```

The script will:
1. List available revisions
2. Allow you to select a revision to rollback to
3. Update traffic routing
4. Verify health

### Manual Rollback

List available revisions:

```bash
gcloud run revisions list \
  --service compliancesync-api \
  --region us-central1
```

Rollback to a specific revision:

```bash
gcloud run services update-traffic compliancesync-api \
  --region us-central1 \
  --to-revisions REVISION-NAME=100
```

### Gradual Traffic Split

For safer rollbacks, gradually shift traffic:

```bash
# 90% to old revision, 10% to new
gcloud run services update-traffic compliancesync-api \
  --region us-central1 \
  --to-revisions OLD-REVISION=90,NEW-REVISION=10
```

---

## Monitoring and Logs

### View Live Logs

Use the logs script:

```bash
cd scripts
./view-logs.sh
```

Or use gcloud directly:

```bash
# Stream logs
gcloud run services logs tail compliancesync-api --region us-central1

# View recent logs
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=compliancesync-api" \
  --limit 100 \
  --format json
```

### Cloud Console

1. Go to [Cloud Console](https://console.cloud.google.com)
2. Navigate to **Cloud Run** → **compliancesync-api**
3. Click **Logs** tab for real-time logs
4. Click **Metrics** tab for performance metrics

### Set Up Alerts

Create alerting policies in Cloud Monitoring:

```bash
# Create uptime check
gcloud monitoring uptime create \
  --display-name "ComplianceSync Health Check" \
  --resource-type uptime-url \
  --resource-labels host=YOUR-SERVICE-URL
```

Recommended alerts:
- Error rate > 1%
- 95th percentile latency > 2s
- No running instances for > 5 minutes
- Memory utilization > 80%

---

## Troubleshooting

### Pipeline Failures

#### "Authentication failed"

**Problem**: Workload Identity Federation not configured correctly

**Solution**:
1. Verify GitHub secrets are set correctly
2. Check service account has required permissions
3. Verify attribute condition in WIF provider matches your repository

```bash
# List workload identity pools
gcloud iam workload-identity-pools list --location global

# Describe provider
gcloud iam workload-identity-pools providers describe github-provider \
  --workload-identity-pool github-pool \
  --location global
```

#### "Permission denied to push to Artifact Registry"

**Problem**: Service account lacks artifact registry writer role

**Solution**:
```bash
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:SERVICE_ACCOUNT_EMAIL" \
  --role="roles/artifactregistry.writer"
```

#### "Tests failed"

**Problem**: Go tests are failing

**Solution**:
1. Run tests locally: `go test ./...`
2. Fix failing tests
3. Commit and push changes

### Deployment Failures

#### "Service deployment failed"

**Problem**: Cloud Run service deployment error

**Solution**:
1. Check service account permissions
2. Verify environment variables are set
3. Check secret values exist in Secret Manager
4. Review Cloud Run logs for errors

```bash
# Check service status
gcloud run services describe compliancesync-api --region us-central1

# View deployment logs
gcloud logging read "resource.type=cloud_run_revision" --limit 50
```

#### "Health check failed"

**Problem**: Service deployed but not responding

**Solution**:
1. Check if service is running: `gcloud run services list`
2. View logs: `gcloud run services logs tail compliancesync-api`
3. Verify Firestore and Storage are accessible
4. Check environment variables are set correctly

### Common Issues

#### Issue: "Firestore: permission denied"

**Solution**: Grant datastore.user role to service account

```bash
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:SERVICE_ACCOUNT_EMAIL" \
  --role="roles/datastore.user"
```

#### Issue: "Cloud Storage: signed URL generation failed"

**Solution**: Grant storage.objectAdmin role to service account

```bash
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:SERVICE_ACCOUNT_EMAIL" \
  --role="roles/storage.objectAdmin"
```

#### Issue: "Secret not found"

**Solution**: Create the secret and add a version

```bash
# Create secret
gcloud secrets create stripe-secret-key --replication-policy automatic

# Add secret value
echo -n "your-secret-value" | gcloud secrets versions add stripe-secret-key --data-file=-

# Grant access
gcloud secrets add-iam-policy-binding stripe-secret-key \
  --member="serviceAccount:SERVICE_ACCOUNT_EMAIL" \
  --role="roles/secretmanager.secretAccessor"
```

---

## Best Practices

### Development Workflow

1. **Create feature branch**
   ```bash
   git checkout -b feature/new-feature
   ```

2. **Develop and test locally**
   ```bash
   go test ./...
   go run cmd/api/main.go
   ```

3. **Commit changes**
   ```bash
   git add .
   git commit -m "feat: add new feature"
   ```

4. **Push to GitHub**
   ```bash
   git push origin feature/new-feature
   ```

5. **Create Pull Request**
   - Tests will run automatically
   - Review changes
   - Merge to main when approved

6. **Automatic deployment to production**
   - Triggered when PR is merged to main

### Security Best Practices

1. **Never commit secrets**: Use Secret Manager
2. **Use Workload Identity Federation**: Avoid service account keys
3. **Least privilege**: Grant minimum required permissions
4. **Review IAM regularly**: Audit service account permissions
5. **Enable Binary Authorization**: For production environments
6. **Use VPC connectors**: If accessing resources in VPC

### Performance Optimization

1. **Adjust instance settings**: Based on load
2. **Enable request concurrency**: Optimize for your workload
3. **Use Cloud CDN**: For static content
4. **Implement caching**: Redis or Memorystore
5. **Monitor metrics**: Adjust based on actual usage

### Cost Optimization

1. **Use min_instances=0**: For non-production environments
2. **Set appropriate timeouts**: Avoid long-running requests
3. **Right-size memory**: Start small, scale up if needed
4. **Use lifecycle policies**: For Cloud Storage buckets
5. **Monitor costs**: Set up budget alerts

---

## Additional Resources

- [Cloud Run Documentation](https://cloud.google.com/run/docs)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Terraform GCP Provider](https://registry.terraform.io/providers/hashicorp/google/latest/docs)
- [Go Testing Best Practices](https://golang.org/pkg/testing/)

---

## Support

For issues or questions:
1. Check this guide and [DEPLOYMENT.md](./DEPLOYMENT.md)
2. Review [Troubleshooting](#troubleshooting) section
3. Check Cloud Run logs and metrics
4. Review GitHub Actions workflow logs

---

**Last Updated**: 2025-10-30
