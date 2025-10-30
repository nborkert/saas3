# ComplianceSync - Terraform Infrastructure

This directory contains Terraform configuration files for provisioning GCP infrastructure for ComplianceSync.

## Prerequisites

- Terraform 1.0 or later
- GCP account with appropriate permissions
- `gcloud` CLI installed and authenticated

## Quick Start

### 1. Initialize Terraform

```bash
cd terraform
terraform init
```

### 2. Create Variables File

```bash
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars` with your values:

```hcl
project_id        = "your-gcp-project-id"
region            = "us-central1"
environment       = "development"
github_repository = "your-github-username/saas3"
```

### 3. Review Planned Changes

```bash
terraform plan
```

### 4. Apply Configuration

```bash
terraform apply
```

Type `yes` when prompted to confirm.

### 5. Create Firestore Database

Terraform doesn't fully support Firestore creation. Create it manually:

```bash
gcloud firestore databases create \
  --location=us-central1 \
  --type=firestore-native \
  --project=your-gcp-project-id
```

### 6. Add Secret Values

After applying, add your secret values:

```bash
# Stripe secret key
echo -n "your-stripe-secret-key" | gcloud secrets versions add stripe-secret-key --data-file=-

# SendGrid API key
echo -n "your-sendgrid-api-key" | gcloud secrets versions add sendgrid-api-key --data-file=-
```

### 7. Configure GitHub Secrets

After applying, Terraform will output the values you need for GitHub Secrets:

```bash
terraform output github_secrets_configuration
```

Add these to your GitHub repository secrets.

## Outputs

After applying, you can view important outputs:

```bash
# View all outputs
terraform output

# View specific output
terraform output service_account_email
```

## Environments

Create separate workspaces for each environment:

```bash
# Development
terraform workspace new development
terraform workspace select development
terraform apply -var="environment=development" -var="min_instances=0" -var="max_instances=5"

# Staging
terraform workspace new staging
terraform workspace select staging
terraform apply -var="environment=staging" -var="min_instances=0" -var="max_instances=10"

# Production
terraform workspace new production
terraform workspace select production
terraform apply -var="environment=production" -var="min_instances=1" -var="max_instances=50"
```

## State Management

For production use, configure a GCS backend for Terraform state:

1. Create a GCS bucket for state:

```bash
gcloud storage buckets create gs://your-terraform-state-bucket --location=us-central1
```

2. Enable versioning:

```bash
gcloud storage buckets update gs://your-terraform-state-bucket --versioning
```

3. Uncomment the backend configuration in `main.tf`:

```hcl
backend "gcs" {
  bucket = "your-terraform-state-bucket"
  prefix = "terraform/state"
}
```

4. Reinitialize Terraform:

```bash
terraform init -migrate-state
```

## Resources Created

This configuration creates:

- **Service Account**: `compliancesync-{environment}@{project-id}.iam.gserviceaccount.com`
- **Cloud Storage Bucket**: `{project-id}-compliancesync-evidence-{environment}`
- **Artifact Registry**: `compliancesync-{environment}`
- **Pub/Sub Topics**:
  - `evidence-processing`
  - `report-generation`
  - `integration-sync`
- **Pub/Sub Subscriptions** (one for each topic)
- **Secret Manager Secrets**:
  - `stripe-secret-key`
  - `sendgrid-api-key`
- **Workload Identity Federation**:
  - Pool: `github-pool`
  - Provider: `github-provider`

## Cleanup

To destroy all resources:

```bash
terraform destroy
```

**Warning**: This will delete all resources created by Terraform, including data in Cloud Storage buckets.

## Troubleshooting

### "API not enabled" errors

If you see errors about APIs not being enabled, wait a few minutes after the first `terraform apply` and run it again. API enablement can take some time to propagate.

### "Workload Identity Pool already exists"

If you're re-creating infrastructure, you may need to import existing resources:

```bash
terraform import google_iam_workload_identity_pool.github_pool projects/{project-id}/locations/global/workloadIdentityPools/github-pool
```

### Firestore Database Issues

Remember that Firestore database creation must be done manually:

```bash
gcloud firestore databases create --location=us-central1 --type=firestore-native
```

## Best Practices

1. **Use workspaces** for different environments
2. **Enable state locking** with GCS backend
3. **Version control** your `.tfvars` files (encrypted) or use secret management
4. **Review plans** carefully before applying
5. **Tag resources** appropriately for cost tracking
6. **Use terraform fmt** to format your code

## Additional Resources

- [Terraform GCP Provider Documentation](https://registry.terraform.io/providers/hashicorp/google/latest/docs)
- [GCP Workload Identity Federation](https://cloud.google.com/iam/docs/workload-identity-federation)
- [Terraform Workspaces](https://www.terraform.io/docs/language/state/workspaces.html)
