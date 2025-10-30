# ComplianceSync Deployment Scripts

This directory contains helper scripts for deploying and managing the ComplianceSync application on Google Cloud Platform.

## Scripts Overview

### 1. setup-gcp-project.sh

**Purpose**: Complete GCP project setup and infrastructure provisioning

**What it does**:
- Enables all required GCP APIs
- Creates Firestore database
- Creates Cloud Storage bucket with versioning
- Creates Artifact Registry repository
- Creates service account with appropriate IAM roles
- Sets up Secret Manager secrets
- Creates Pub/Sub topics and subscriptions
- Configures Workload Identity Federation for GitHub Actions
- Displays GitHub secrets configuration

**Usage**:
```bash
./setup-gcp-project.sh
```

**When to use**:
- First-time setup for a new environment
- Creating dev, staging, or production environments

---

### 2. deploy-manual.sh

**Purpose**: Manual deployment bypassing GitHub Actions

**What it does**:
- Builds Docker image locally
- Pushes to Artifact Registry
- Deploys to Cloud Run
- Runs health checks

**Usage**:
```bash
./deploy-manual.sh
```

**When to use**:
- CI/CD pipeline is unavailable
- Testing deployments locally
- Emergency hotfix deployments
- Development testing

---

### 3. rollback.sh

**Purpose**: Rollback Cloud Run service to a previous revision

**What it does**:
- Lists available service revisions
- Allows selection of revision to rollback to
- Updates traffic routing
- Verifies health after rollback

**Usage**:
```bash
./rollback.sh
```

**When to use**:
- Deployment caused issues
- Need to revert to a known good state
- Testing traffic splitting strategies

---

### 4. view-logs.sh

**Purpose**: View Cloud Run service logs

**What it does**:
- Streams real-time logs
- Views recent logs
- Filters logs by severity
- Displays structured log output

**Usage**:
```bash
./view-logs.sh
```

**When to use**:
- Debugging issues
- Monitoring application behavior
- Investigating errors
- Auditing system activity

---

### 5. seed-requirements.sh

**Purpose**: Seed Firestore with regulatory requirement templates

**What it does**:
- Generates sample requirement template data
- Provides instructions for seeding Firestore
- Creates JSON file with template data

**Usage**:
```bash
./seed-requirements.sh
```

**When to use**:
- Initial database setup
- Testing with sample data
- Loading production requirement templates

---

### 6. validate-setup.sh

**Purpose**: Validate that GCP infrastructure is properly configured

**What it does**:
- Checks all required APIs are enabled
- Verifies Firestore database exists
- Validates Cloud Storage bucket configuration
- Checks Artifact Registry repository
- Verifies service account and IAM roles
- Validates Secret Manager secrets
- Checks Pub/Sub topics and subscriptions
- Verifies Workload Identity Federation

**Usage**:
```bash
./validate-setup.sh
```

**When to use**:
- After running setup script
- Before first deployment
- Troubleshooting configuration issues
- Auditing environment setup

---

## Quick Start Guide

### First-Time Setup

1. **Run the setup script**:
   ```bash
   ./setup-gcp-project.sh
   ```
   Follow the prompts to select environment and enter project details.

2. **Validate the setup**:
   ```bash
   ./validate-setup.sh
   ```
   Ensure all checks pass.

3. **Configure GitHub Secrets**:
   - Copy the secret values from the setup script output
   - Add them to your GitHub repository (Settings → Secrets → Actions)

4. **Deploy via GitHub Actions**:
   - Push to `main` branch for automatic deployment
   - Or use manual workflow trigger in GitHub Actions

### Emergency Deployment

If GitHub Actions is unavailable:

1. **Deploy manually**:
   ```bash
   ./deploy-manual.sh
   ```

2. **Verify deployment**:
   ```bash
   ./view-logs.sh
   ```

### Rollback After Bad Deployment

1. **Execute rollback**:
   ```bash
   ./rollback.sh
   ```

2. **Select previous working revision**

3. **Verify health check passes**

## Prerequisites

All scripts require:
- `gcloud` CLI installed and authenticated
- `docker` installed (for deployment scripts)
- `git` installed (for deployment scripts)
- Appropriate GCP project permissions

## Environment Variables

Some scripts respect environment variables:

```bash
# Set project ID
export PROJECT_ID=your-project-id

# Set region
export REGION=us-central1

# Set environment
export ENVIRONMENT=production
```

## Troubleshooting

### "Permission denied" when running scripts

Make scripts executable:
```bash
chmod +x *.sh
```

### "gcloud: command not found"

Install Google Cloud SDK:
```bash
# macOS
brew install --cask google-cloud-sdk

# Linux
curl https://sdk.cloud.google.com | bash
```

### "Authentication failed"

Authenticate with gcloud:
```bash
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
```

### "API not enabled"

Run the setup script which enables all required APIs:
```bash
./setup-gcp-project.sh
```

## Best Practices

1. **Always validate after setup**:
   ```bash
   ./setup-gcp-project.sh
   ./validate-setup.sh
   ```

2. **Test in development first**:
   - Set up development environment
   - Deploy and test
   - Then set up staging/production

3. **Keep scripts updated**:
   - Scripts are version controlled
   - Pull latest changes before using

4. **Use manual deployment sparingly**:
   - Prefer GitHub Actions for traceability
   - Use manual deployment only when necessary

5. **Always check logs after deployment**:
   ```bash
   ./view-logs.sh
   ```

## Additional Resources

- [CICD_GUIDE.md](../CICD_GUIDE.md) - Complete CI/CD documentation
- [DEPLOYMENT.md](../DEPLOYMENT.md) - Manual deployment guide
- [README.md](../README.md) - Project overview

## Support

For issues with scripts:
1. Check script output for error messages
2. Run validation script to identify missing resources
3. Review logs for detailed error information
4. Check GCP Console for resource status
