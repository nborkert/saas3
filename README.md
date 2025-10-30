# ComplianceSync API

A production-ready Go backend for the ComplianceSync SaaS platform - a compliance evidence management system for small regulated businesses.

## Overview

ComplianceSync automates compliance documentation by capturing evidence from workplace tools (Google Workspace, Microsoft 365), organizing it against regulatory requirement templates, and maintaining audit-ready records. This backend implements authentication, multi-tenant data isolation, evidence management, and compliance reporting.

## Architecture

- **Language**: Go 1.21+
- **Database**: Google Cloud Firestore (multi-tenant document store)
- **Authentication**: Firebase Identity Platform (JWT-based with custom claims)
- **File Storage**: Google Cloud Storage (signed URLs for secure uploads/downloads)
- **Background Jobs**: Pub/Sub + Cloud Run workers
- **Payment Processing**: Stripe API integration
- **Deployment**: Cloud Run (serverless containers)

## Features Implemented

### P0 (Critical) User Stories

- **STORY-001**: User registration with email verification
- **STORY-002**: Secure user login with session management
- **STORY-003**: Organization profile setup
- **STORY-004**: Subscription tier selection
- **STORY-005**: Password reset flow
- **STORY-006**: View pre-built regulatory requirement templates
- **STORY-007**: Activate regulatory requirements
- **STORY-008**: Compliance dashboard overview
- **STORY-009**: Requirement detail view
- **STORY-013**: Manual evidence upload with Cloud Storage
- **STORY-014**: Evidence list view and search
- **STORY-020**: Associate evidence with requirements
- **STORY-021**: Evidence deletion with audit trail
- **STORY-022**: Immutable audit logging
- **STORY-023**: Requirement compliance report generation
- **STORY-024**: Comprehensive compliance reporting

### Security Features

- Multi-tenant data isolation (organizationId in custom JWT claims)
- Role-based access control (Admin, Compliance Officer, Viewer)
- Signed URLs for secure file upload/download
- Immutable audit logs
- Input validation and sanitization
- Password security requirements

## Project Structure

```
compliancesync-api/
├── cmd/
│   └── api/
│       └── main.go                 # Application entry point
├── internal/
│   ├── api/
│   │   ├── server.go               # Server initialization and routing
│   │   ├── handlers.go             # Auth and user handlers
│   │   ├── requirements_handlers.go # Regulatory requirements handlers
│   │   ├── evidence_handlers.go    # Evidence management handlers
│   │   ├── audit_reports_handlers.go # Audit logs and reports handlers
│   │   └── webhooks_workers_handlers.go # Webhooks and workers
│   ├── auth/
│   │   └── middleware.go           # Firebase authentication middleware
│   ├── models/
│   │   ├── organization.go         # Organization models
│   │   ├── user.go                 # User and role models
│   │   ├── requirement.go          # Regulatory requirement models
│   │   ├── evidence.go             # Evidence and integration models
│   │   └── audit.go                # Audit log and report models
│   └── store/
│       └── firestore.go            # Firestore database operations
├── workers/
│   └── pdf-generator/              # PDF generation worker (placeholder)
├── Dockerfile                      # Multi-stage Docker build
├── .env.example                    # Example environment configuration
├── go.mod                          # Go module dependencies
└── README.md                       # This file
```

## Prerequisites

- Go 1.21 or later
- GCP account with the following APIs enabled:
  - Cloud Firestore API
  - Cloud Storage API
  - Identity Platform (Firebase Authentication)
  - Cloud Run API
- Firebase project with Identity Platform configured
- GCP service account with appropriate permissions
- Stripe account (for payment processing)
- SendGrid account (for email notifications)

## Local Development Setup

### 1. Clone and Install Dependencies

```bash
cd /Users/nealborkert/Downloads/src/saas3
go mod download
```

### 2. Configure Environment Variables

Copy the example environment file and configure it:

```bash
cp .env.example .env
```

Edit `.env` with your configuration:

```bash
# Required
GCP_PROJECT_ID=your-gcp-project-id
STORAGE_BUCKET=your-storage-bucket-name
GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json

# Optional for local dev
PORT=8080
ENVIRONMENT=development
STRIPE_SECRET_KEY=sk_test_your_stripe_key
SENDGRID_API_KEY=your_sendgrid_api_key
```

### 3. Set Up GCP Resources

#### Create Firestore Database

```bash
gcloud firestore databases create --location=us-central1
```

#### Create Cloud Storage Bucket

```bash
gcloud storage buckets create gs://your-storage-bucket-name --location=us-central1
```

#### Enable Required APIs

```bash
gcloud services enable firestore.googleapis.com
gcloud services enable storage.googleapis.com
gcloud services enable identitytoolkit.googleapis.com
gcloud services enable run.googleapis.com
```

#### Create Service Account

```bash
gcloud iam service-accounts create compliancesync-api \
    --display-name="ComplianceSync API Service Account"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:compliancesync-api@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/datastore.user"

gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:compliancesync-api@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/storage.objectAdmin"

gcloud iam service-accounts keys create credentials.json \
    --iam-account=compliancesync-api@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

### 4. Configure Firebase Identity Platform

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your GCP project
3. Navigate to Authentication → Sign-in method
4. Enable Email/Password authentication
5. Optionally enable Google and Microsoft OAuth providers

### 5. Run Locally

```bash
# Source environment variables
export $(cat .env | xargs)

# Run the application
go run cmd/api/main.go
```

The API will be available at `http://localhost:8080`

## API Endpoints

### Authentication

- `POST /api/v1/auth/register` - Register new user and organization
- `POST /api/v1/auth/password-reset` - Request password reset

### User Profile

- `GET /api/v1/profile` - Get current user profile (requires auth)
- `PUT /api/v1/profile` - Update profile (requires auth)

### Organization Management

- `GET /api/v1/organization` - Get organization details (requires auth)
- `PUT /api/v1/organization` - Update organization (requires admin)
- `GET /api/v1/organization/dashboard` - Get compliance dashboard metrics

### User Management

- `GET /api/v1/users` - List all users
- `POST /api/v1/users/invite` - Invite new user (requires admin)
- `PUT /api/v1/users/{userID}/role` - Update user role (requires admin)
- `DELETE /api/v1/users/{userID}` - Remove user (requires admin)

### Regulatory Requirements

- `GET /api/v1/requirements` - List active requirements
- `GET /api/v1/requirements/templates` - List available templates
- `POST /api/v1/requirements` - Activate requirement from template
- `GET /api/v1/requirements/{requirementID}` - Get requirement details
- `PUT /api/v1/requirements/{requirementID}` - Update requirement
- `DELETE /api/v1/requirements/{requirementID}` - Deactivate requirement

### Evidence Management

- `GET /api/v1/evidence` - List all evidence
- `POST /api/v1/evidence/upload-url` - Generate signed upload URL
- `POST /api/v1/evidence` - Complete evidence upload and associate with requirements
- `GET /api/v1/evidence/{evidenceID}` - Get evidence details
- `PUT /api/v1/evidence/{evidenceID}` - Update evidence
- `DELETE /api/v1/evidence/{evidenceID}` - Delete evidence
- `GET /api/v1/evidence/{evidenceID}/download-url` - Generate signed download URL

### Audit Logs

- `GET /api/v1/audit-logs` - List audit logs (with filters)
- `GET /api/v1/audit-logs/export` - Export audit logs to CSV

### Reports

- `GET /api/v1/reports` - List generated reports
- `POST /api/v1/reports` - Generate new compliance report
- `GET /api/v1/reports/{reportID}` - Get report details
- `GET /api/v1/reports/{reportID}/download-url` - Get report download URL

### Subscription Management

- `GET /api/v1/subscription` - Get subscription details
- `POST /api/v1/subscription` - Create subscription
- `PUT /api/v1/subscription` - Update subscription tier
- `POST /api/v1/subscription/cancel` - Cancel subscription

### Integrations

- `GET /api/v1/integrations` - List integrations
- `POST /api/v1/integrations/google/connect` - Connect Google Workspace
- `DELETE /api/v1/integrations/google/disconnect` - Disconnect Google Workspace

### Health Check

- `GET /health` - Health check endpoint (unauthenticated)

## Authentication

All protected endpoints require a JWT token in the `Authorization` header:

```
Authorization: Bearer <firebase-jwt-token>
```

The JWT must include custom claims:
- `organizationId`: The user's organization ID
- `role`: The user's role (admin, compliance_officer, or viewer)

## Building and Deploying

### Build Docker Image

```bash
docker build -t compliancesync-api:latest .
```

### Deploy to Cloud Run

```bash
# Build and push to Google Container Registry
gcloud builds submit --tag gcr.io/YOUR_PROJECT_ID/compliancesync-api

# Deploy to Cloud Run
gcloud run deploy compliancesync-api \
  --image gcr.io/YOUR_PROJECT_ID/compliancesync-api \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars "GCP_PROJECT_ID=YOUR_PROJECT_ID,STORAGE_BUCKET=your-bucket-name" \
  --set-secrets "STRIPE_SECRET_KEY=stripe-secret-key:latest,SENDGRID_API_KEY=sendgrid-api-key:latest" \
  --service-account compliancesync-api@YOUR_PROJECT_ID.iam.gserviceaccount.com \
  --max-instances 10 \
  --memory 512Mi \
  --timeout 60s
```

## Testing

### Manual Testing with cURL

Register a new user:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "full_name": "Admin User",
    "organization_name": "Example Corp",
    "password": "SecureP@ss123",
    "industry": "financial_services",
    "employee_count": "11-25",
    "regulatory_framework": "sec_ria"
  }'
```

Get dashboard (with auth token):

```bash
curl -X GET http://localhost:8080/api/v1/organization/dashboard \
  -H "Authorization: Bearer YOUR_FIREBASE_JWT_TOKEN"
```

## Multi-Tenant Data Isolation

All data operations enforce tenant isolation:

1. **JWT Custom Claims**: Every authenticated request includes `organizationId` in the JWT
2. **Firestore Structure**: All collections are nested under `/organizations/{orgId}`
3. **Query Filtering**: All database queries filter by `organizationId`
4. **Storage Paths**: Cloud Storage paths are prefixed with `{orgId}/`

## Role-Based Access Control

### Admin Role
- Full access to all features
- Can manage users, billing, and integrations
- Can modify organization settings

### Compliance Officer Role
- Can manage requirements and evidence
- Can generate reports
- Cannot manage users or billing

### Viewer Role
- Read-only access to all compliance data
- Can view dashboard, requirements, and evidence
- Can generate and download reports
- Cannot modify any data

## Audit Logging

All user actions are automatically logged to immutable audit logs:

- User authentication (login, logout)
- Organization changes
- Requirement activations/updates
- Evidence uploads/deletions
- Report generation
- Integration connections
- Subscription changes

Audit logs include:
- Timestamp (server-generated)
- User ID and email
- Action type
- Resource affected
- Description
- IP address and user agent
- Change details (JSON diff for updates)

## Error Handling

All endpoints return consistent JSON error responses:

```json
{
  "error": "error message here"
}
```

HTTP status codes:
- 200: Success
- 201: Created
- 400: Bad request (validation error)
- 401: Unauthorized (authentication required)
- 403: Forbidden (insufficient permissions)
- 404: Not found
- 409: Conflict (e.g., duplicate email)
- 500: Internal server error

## Monitoring and Logging

Application logs are structured JSON (suitable for Cloud Logging):

```json
{
  "level": "info",
  "message": "user registered",
  "user_id": "abc123",
  "organization_id": "org_456",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

## Environment Variables Reference

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `PORT` | No | HTTP server port | `8080` |
| `GCP_PROJECT_ID` | Yes | Google Cloud Project ID | - |
| `GOOGLE_APPLICATION_CREDENTIALS` | Yes | Path to service account JSON | - |
| `STORAGE_BUCKET` | Yes | Cloud Storage bucket name | - |
| `STRIPE_SECRET_KEY` | No | Stripe secret key (for payments) | - |
| `SENDGRID_API_KEY` | No | SendGrid API key (for emails) | - |
| `ENVIRONMENT` | No | Environment name | `development` |

## Next Steps for Production

1. **Add Unit Tests**: Implement unit tests for handlers and store operations
2. **Integration Tests**: Set up integration tests with Firestore emulator
3. **CI/CD Pipeline**: Configure Cloud Build for automated testing and deployment
4. **Monitoring**: Set up Cloud Monitoring dashboards and alerts
5. **Rate Limiting**: Implement API rate limiting with Cloud Armor or in-app
6. **Background Workers**: Complete Pub/Sub worker implementation for:
   - Gmail/Drive polling (STORY-016, STORY-017)
   - PDF report generation (STORY-023, STORY-024)
   - Email notifications (STORY-044)
7. **Requirement Templates**: Seed Firestore with actual regulatory requirement templates (STORY-010, STORY-011, STORY-012)
8. **OAuth Flows**: Complete Google and Microsoft OAuth integration flows
9. **Stripe Integration**: Implement full Stripe subscription lifecycle
10. **SendGrid Templates**: Create email templates for notifications

## Troubleshooting

### "failed to initialize firestore client"
- Verify `GCP_PROJECT_ID` is set correctly
- Check service account has `roles/datastore.user` permission
- Ensure Firestore API is enabled

### "failed to generate signed URL"
- Verify `STORAGE_BUCKET` exists
- Check service account has `roles/storage.objectAdmin` permission
- Ensure Cloud Storage API is enabled

### "invalid or expired token"
- Verify Firebase project ID matches `GCP_PROJECT_ID`
- Check JWT token is not expired
- Ensure custom claims are set correctly

## CI/CD Pipeline

### Automated Deployment

This project includes a production-ready CI/CD pipeline using GitHub Actions with Workload Identity Federation for secure, keyless authentication to GCP.

#### Quick Start

1. **Set up GCP infrastructure** (one-time per environment):
   ```bash
   cd scripts
   ./setup-gcp-project.sh
   ```

2. **Configure GitHub Secrets** (from setup script output):
   - Navigate to GitHub repository Settings → Secrets and variables → Actions
   - Add the secrets displayed by the setup script

3. **Deploy automatically**:
   - Push to `main` branch for automatic production deployment
   - Or use GitHub Actions → Run workflow for manual deployment to any environment

#### Pipeline Features

- **Automated Testing**: Runs Go tests on every commit
- **Multi-Environment Support**: Development, Staging, Production
- **Immutable Artifacts**: Container images tagged with Git SHA
- **Health Checks**: Automatic verification of deployments
- **Easy Rollback**: Quick rollback to previous revisions

#### Available Scripts

```bash
# Complete GCP project setup (interactive)
./scripts/setup-gcp-project.sh

# Manual deployment (bypasses GitHub Actions)
./scripts/deploy-manual.sh

# Rollback to previous revision
./scripts/rollback.sh

# View and stream logs
./scripts/view-logs.sh

# Seed requirement templates
./scripts/seed-requirements.sh
```

#### Terraform (Infrastructure as Code)

Alternatively, use Terraform for reproducible infrastructure:

```bash
cd terraform
terraform init
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars with your values
terraform plan
terraform apply
```

For detailed CI/CD documentation, see [CICD_GUIDE.md](./CICD_GUIDE.md).

For manual deployment steps, see [DEPLOYMENT.md](./DEPLOYMENT.md).

## Contributing

Follow these guidelines:
- Write clear commit messages
- Add tests for new features
- Update documentation for API changes
- Use `go fmt` for code formatting
- Run `go vet` to catch common errors

## License

Proprietary - All rights reserved

---

**Disclaimer**: This generated code provides a functional implementation of the specified user stories and architecture. However, it requires comprehensive testing, security hardening, and peer review before being deployed to production. Ensure you:
- Add unit and integration tests
- Conduct security audits
- Implement monitoring and alerting
- Review error handling for production scenarios
- Add appropriate database migrations
- Configure proper CI/CD pipelines
- Set up proper secrets management
