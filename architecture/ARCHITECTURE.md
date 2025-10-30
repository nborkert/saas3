# Architectural Decision Record: ComplianceSync on GCP

## 1. Executive Summary

ComplianceSync is a multi-tenant SaaS platform designed to streamline compliance documentation for small businesses. This architecture leverages GCP's managed services and serverless technologies to minimize operational overhead while maximizing security, scalability, and developer velocity.

The design philosophy centers on **security-first multi-tenancy**, **serverless compute**, and **cost-efficient scaling**. For an early-stage SaaS targeting 90-1000 organizations, we prioritize managed services that eliminate infrastructure management, provide built-in scaling, and offer pay-per-use pricing models. The architecture uses Cloud Run for the application backend, Firestore for flexible multi-tenant data storage, Cloud Storage for evidence files, Cloud Scheduler + Pub/Sub for background jobs, and Identity Platform for authentication. This combination provides enterprise-grade security with startup-friendly operational simplicity.

Key architectural principles include: tenant-level data isolation through database design patterns, immutable audit trails using append-only logs, secure file storage with signed URLs, and integration-ready APIs using OAuth 2.0. The system is designed to scale horizontally and automatically, with costs that grow linearly with actual usage rather than pre-provisioned capacity.

## 2. Core Components & Service Mapping

| Application Requirement | Selected GCP Service | Rationale |
|------------------------|---------------------|----------|
| Application Backend (REST API) | **Cloud Run** | Serverless container platform with auto-scaling from 0 to N instances. Ideal for REST APIs with variable traffic. Pay only for actual request processing time. Supports any language/framework. Better than Cloud Functions for complex business logic and existing frameworks (Express, FastAPI, Spring Boot). Better than GKE due to significantly lower operational complexity for early-stage teams. |
| Primary Database | **Firestore (Native Mode)** | Document database with strong multi-tenancy support via collection-based isolation. Excellent for flexible schemas (varying compliance frameworks), built-in real-time capabilities, automatic scaling, and strong consistency. Simpler than Cloud SQL for document-heavy workloads with hierarchical data (organizations → frameworks → evidence → audit logs). No connection pooling or migration management required. Better query flexibility for tenant-isolated data than Bigtable. |
| File Storage (Evidence Documents) | **Cloud Storage** | Object storage designed for large files. Supports files up to 5TB (your 25MB limit is well within range). Provides signed URLs for secure temporary access, versioning for compliance requirements, and lifecycle policies for cost optimization. Integration with Cloud CDN for global delivery if needed. More cost-effective than storing files in Firestore or Cloud SQL. |
| Authentication & User Management | **Identity Platform** | Firebase Authentication with enterprise features. Supports email/password, OAuth providers (Google, Microsoft), and custom claims for tenant isolation. Handles password resets, email verification, and session management. Integrates seamlessly with Cloud Run via Firebase Admin SDK. More feature-complete than implementing custom auth on Cloud Identity. Cheaper than third-party solutions like Auth0 at your scale. |
| Background Job Scheduling | **Cloud Scheduler** | Managed cron service that triggers jobs at specified intervals (your 15-30 min polling requirement). Serverless, highly reliable, and integrates directly with Pub/Sub. No need to maintain worker processes or polling logic. |
| Background Job Queue & Processing | **Pub/Sub + Cloud Run** | Pub/Sub provides reliable message queuing with at-least-once delivery. Cloud Run workers subscribe to topics and process jobs asynchronously. This pattern handles: Gmail/Drive polling, report generation, email notifications. More flexible than Cloud Tasks for fan-out patterns and decoupled architectures. Better than standalone Cloud Functions due to longer execution time limits (60 min vs 9 min) for PDF generation. |
| Email Notifications | **SendGrid via Cloud Run** | SendGrid (or similar SMTP service) called from Cloud Run provides transactional email with delivery tracking. GCP doesn't offer a native email service. SendGrid free tier supports 100 emails/day, paid plans scale affordably. Alternative: Mailgun, AWS SES via API. |
| Audit Logging (Immutable) | **Firestore + Cloud Logging** | Application-level audit logs stored in Firestore subcollections with security rules preventing deletion. System-level logs (API access, errors) in Cloud Logging with retention policies. Firestore provides queryable audit trails per tenant. Cloud Logging captures infrastructure and security events. More cost-effective than BigQuery for write-heavy audit workloads at your scale. |
| Payment Processing | **Stripe API via Cloud Run** | Stripe handles PCI compliance, subscriptions, and invoicing. Integration via Stripe SDK in Cloud Run backend. Webhooks received at Cloud Run endpoints trigger subscription state changes in Firestore. Keep card data entirely in Stripe (never store in your infrastructure). |
| Secrets Management | **Secret Manager** | Centralized secret storage for API keys (Stripe, SendGrid, OAuth credentials). Integrates with Cloud Run via environment variables or runtime access. Automatic rotation support. Versioning for rollback. Better security than environment variables alone. |
| OAuth Integration Storage | **Firestore Encrypted Fields** | Store OAuth refresh tokens encrypted in Firestore using Cloud KMS. Each tenant's integration credentials isolated in their document structure. Allows background jobs to access Gmail/Drive on behalf of users. |
| PDF Report Generation | **Cloud Run + Puppeteer/wkhtmltopdf** | CPU-intensive PDF generation runs in Cloud Run workers triggered by Pub/Sub. Longer execution timeout (60 min) supports complex reports. Generated PDFs stored in Cloud Storage. Alternative: Consider Gotenberg (Docker-based PDF service) or headless Chrome in container. |
| API Gateway (Optional, Recommended) | **Cloud Endpoints or API Gateway** | Provides API key management, rate limiting, and request validation before reaching Cloud Run. Useful for public API integrations and monitoring. Optional for MVP; recommended for production scale. |
| Monitoring & Alerting | **Cloud Monitoring & Cloud Logging** | Built-in observability for all GCP services. Set up alerts for error rates, latency, and cost thresholds. Log-based metrics for custom application events. Integrates with PagerDuty/Slack for on-call. |

## 3. Detailed Service Integration Guide (For Developers)

### Component 1: Authentication & User Management
- **GCP Service**: Identity Platform
- **Developer Guidance**:
  - **SDK**: Use Firebase Admin SDK (Node.js: `firebase-admin`, Python: `firebase-admin`, Java: `com.google.firebase:firebase-admin`)
  - **Client-side**: Firebase Auth SDK for web/mobile apps
  - **Authentication Pattern**:
    - Users sign up/login via Identity Platform (email/password or OAuth)
    - Identity Platform returns JWT tokens
    - Client sends JWT in `Authorization: Bearer <token>` header
    - Cloud Run verifies JWT using Firebase Admin SDK: `admin.auth().verifyIdToken(token)`
  - **Multi-tenancy**: Add custom claims to JWT with `organizationId`:
    ```javascript
    await admin.auth().setCustomUserClaims(uid, { organizationId: 'org_123' });
    ```
  - **Environment Variables**: Store `GOOGLE_APPLICATION_CREDENTIALS` for service account access
  - **Security**: Enable multi-factor authentication (MFA) in Identity Platform settings for enterprise customers. Use Firebase Security Rules to enforce tenant isolation at the client level (if using Firestore directly from frontend).
  - **OAuth Integration**: Configure Google and Microsoft OAuth providers in Identity Platform console. Use OAuth refresh tokens for background Gmail/Drive access (store securely in Firestore with encryption).

### Component 2: Application Backend (REST API)
- **GCP Service**: Cloud Run
- **Developer Guidance**:
  - **Framework**: Use any framework you prefer (Express.js, FastAPI, Django, Spring Boot). Containerize with Docker.
  - **Deployment**:
    ```bash
    gcloud run deploy compliancesync-api \
      --source . \
      --region us-central1 \
      --allow-unauthenticated \
      --set-env-vars "FIRESTORE_PROJECT_ID=your-project"
    ```
  - **Authentication Middleware**: Create middleware to verify JWT and extract `organizationId`:
    ```javascript
    async function authenticateRequest(req, res, next) {
      const token = req.headers.authorization?.split('Bearer ')[1];
      const decodedToken = await admin.auth().verifyIdToken(token);
      req.user = { uid: decodedToken.uid, organizationId: decodedToken.organizationId };
      next();
    }
    ```
  - **Tenant Isolation**: Every database query must filter by `organizationId` from the JWT. Never trust client-provided tenant IDs.
  - **Scaling**: Configure `--min-instances 0` (scale to zero for cost savings) and `--max-instances 10` (prevent runaway costs). Adjust concurrency with `--concurrency 80`.
  - **Local Development**: Run locally with `docker build` and `docker run`, or use Cloud Code VSCode extension for live debugging against Firestore emulator.
  - **Health Checks**: Implement `/health` endpoint for Cloud Run readiness checks.

### Component 3: Primary Database (Multi-tenant Data)
- **GCP Service**: Firestore (Native Mode)
- **Developer Guidance**:
  - **SDK**: Use official SDKs (Node.js: `@google-cloud/firestore`, Python: `google-cloud-firestore`)
  - **Multi-tenancy Pattern**: Use collection structure with tenant ID as the primary key:
    ```
    /organizations/{organizationId}
      /frameworks/{frameworkId}
        /evidence/{evidenceId}
          /auditLogs/{logId}
      /users/{userId}
      /integrations/{integrationType}
    ```
  - **Tenant Isolation Code Example**:
    ```javascript
    const db = admin.firestore();
    const orgRef = db.collection('organizations').doc(req.user.organizationId);
    const evidenceDocs = await orgRef.collection('evidence').where('status', '==', 'active').get();
    ```
  - **Indexes**: Create composite indexes for common queries (e.g., `organizationId + createdAt`). Firestore will prompt you with index creation links when queries fail locally.
  - **Security Rules**: Deploy Firestore Security Rules to enforce tenant isolation even if client accesses Firestore directly:
    ```javascript
    match /organizations/{orgId} {
      allow read, write: if request.auth.token.organizationId == orgId;
    }
    ```
  - **Transactions**: Use transactions for operations requiring consistency (e.g., subscription updates with audit logs).
  - **Connection**: Firestore uses HTTP/2 and automatically manages connections. No connection pooling required.

### Component 4: File Storage (Evidence Documents)
- **GCP Service**: Cloud Storage
- **Developer Guidance**:
  - **SDK**: Use `@google-cloud/storage` (Node.js) or `google-cloud-storage` (Python)
  - **Bucket Structure**: Create tenant-isolated paths within a single bucket:
    ```
    gs://compliancesync-evidence-prod/
      org_123/evidence/evidence_abc.pdf
      org_456/evidence/evidence_xyz.docx
    ```
  - **Upload Pattern (Signed URLs)**:
    1. Client requests upload URL from Cloud Run API
    2. Cloud Run generates signed URL with `storage.bucket().file().getSignedUrl()`
    3. Client uploads directly to Cloud Storage using signed URL (bypasses backend)
    4. Client notifies backend of completion; backend stores metadata in Firestore
  - **Code Example**:
    ```javascript
    const bucket = storage.bucket('compliancesync-evidence-prod');
    const filePath = `${organizationId}/evidence/${evidenceId}.pdf`;
    const file = bucket.file(filePath);
    const [signedUrl] = await file.getSignedUrl({
      version: 'v4',
      action: 'write',
      expires: Date.now() + 15 * 60 * 1000, // 15 minutes
      contentType: 'application/pdf'
    });
    ```
  - **Download Pattern**: Generate signed read URLs with expiration (e.g., 1 hour) to provide secure access without making bucket public.
  - **Security**: Enable uniform bucket-level access. Use IAM roles, not ACLs. Grant Cloud Run service account `roles/storage.objectAdmin`. Never make bucket public.
  - **Lifecycle Policies**: Configure automatic deletion of files after 7 years (or per compliance retention policy).
  - **Versioning**: Enable object versioning for compliance requirements (track evidence modifications).

### Component 5: Background Job Scheduling & Processing
- **GCP Services**: Cloud Scheduler + Pub/Sub + Cloud Run
- **Developer Guidance**:
  - **Scheduler Setup**: Create cron jobs for each integration polling task:
    ```bash
    gcloud scheduler jobs create pubsub poll-gmail-job \
      --schedule "*/15 * * * *" \
      --topic gmail-poll-topic \
      --message-body '{"action":"poll_gmail"}'
    ```
  - **Pub/Sub Topics**: Create topics for different job types:
    - `gmail-poll-topic`: Poll Gmail for new evidence emails
    - `drive-poll-topic`: Poll Google Drive for document changes
    - `pdf-generation-topic`: Generate compliance reports
    - `notification-topic`: Send email notifications
  - **Worker Service**: Deploy separate Cloud Run services as Pub/Sub subscribers:
    ```bash
    gcloud run deploy gmail-worker \
      --source ./workers/gmail \
      --no-allow-unauthenticated \
      --set-env-vars "PUBSUB_VERIFICATION=enabled"
    ```
  - **Pub/Sub Push Subscription**: Configure Pub/Sub to push messages to Cloud Run worker endpoints:
    ```bash
    gcloud pubsub subscriptions create gmail-poll-sub \
      --topic gmail-poll-topic \
      --push-endpoint https://gmail-worker-xxx.run.app/process \
      --push-auth-service-account worker@project.iam.gserviceaccount.com
    ```
  - **Worker Code Pattern**:
    ```javascript
    app.post('/process', async (req, res) => {
      const message = Buffer.from(req.body.message.data, 'base64').toString();
      const { action } = JSON.parse(message);

      // Fetch all organizations that have Gmail integration enabled
      const orgsSnapshot = await db.collection('organizations')
        .where('integrations.gmail.enabled', '==', true).get();

      for (const orgDoc of orgsSnapshot.docs) {
        await pollGmailForOrg(orgDoc.id);
      }

      res.status(200).send('OK'); // Acknowledge message
    });
    ```
  - **Idempotency**: Implement idempotency keys in Firestore to prevent duplicate processing (Pub/Sub guarantees at-least-once delivery).
  - **Error Handling**: Let exceptions propagate to return 500. Pub/Sub will retry with exponential backoff. Configure dead-letter topics for failed messages.
  - **OAuth Token Refresh**: Retrieve encrypted OAuth tokens from Firestore, refresh if expired using Google OAuth2 client library, and poll APIs.

### Component 6: Audit Logging (Immutable)
- **GCP Services**: Firestore + Cloud Logging
- **Developer Guidance**:
  - **Application Audit Logs**: Store in Firestore subcollections under each entity:
    ```
    /organizations/{orgId}/evidence/{evidenceId}/auditLogs/{logId}
      - timestamp (server timestamp)
      - userId (who performed action)
      - action ("created", "updated", "deleted", "viewed", "exported")
      - changes (JSON diff of what changed)
      - ipAddress
      - userAgent
    ```
  - **Code Example**:
    ```javascript
    await orgRef.collection('evidence').doc(evidenceId).collection('auditLogs').add({
      timestamp: admin.firestore.FieldValue.serverTimestamp(),
      userId: req.user.uid,
      action: 'updated',
      changes: { status: { from: 'draft', to: 'published' } },
      ipAddress: req.ip,
      userAgent: req.headers['user-agent']
    });
    ```
  - **Immutability**: Deploy Firestore Security Rules to prevent deletion:
    ```javascript
    match /organizations/{orgId}/evidence/{evidenceId}/auditLogs/{logId} {
      allow create: if request.auth.token.organizationId == orgId;
      allow read: if request.auth.token.organizationId == orgId;
      allow update, delete: if false; // No updates or deletions
    }
    ```
  - **System Audit Logs**: Cloud Logging automatically captures:
    - Admin activity (IAM changes, project modifications)
    - Data access (Cloud Storage file access, Firestore queries if enabled)
    - System events (Cloud Run deployments, errors)
  - **Retention**: Configure log retention in Cloud Logging (default 30 days). Export to Cloud Storage for long-term retention (7+ years for compliance).
  - **Querying**: Use Firestore queries for application audit trails. Use Logs Explorer for system logs.

### Component 7: Payment Processing
- **Integration**: Stripe API via Cloud Run
- **Developer Guidance**:
  - **SDK**: Use official Stripe SDK (`stripe` package for Node.js/Python)
  - **Environment Variables**: Store Stripe secret key in Secret Manager, access in Cloud Run
  - **Subscription Flow**:
    1. Client initiates subscription upgrade via frontend
    2. Frontend calls Cloud Run API endpoint: `POST /api/subscriptions`
    3. Cloud Run creates Stripe subscription: `stripe.subscriptions.create()`
    4. Store subscription ID and status in Firestore: `/organizations/{orgId}/subscription`
  - **Webhook Handling**: Stripe sends webhook events to Cloud Run endpoint:
    ```javascript
    app.post('/webhooks/stripe', async (req, res) => {
      const sig = req.headers['stripe-signature'];
      const event = stripe.webhooks.constructEvent(req.body, sig, webhookSecret);

      if (event.type === 'customer.subscription.updated') {
        await db.collection('organizations').doc(customerId).update({
          'subscription.status': event.data.object.status,
          'subscription.currentPeriodEnd': new Date(event.data.object.current_period_end * 1000)
        });
      }

      res.json({ received: true });
    });
    ```
  - **Security**: Verify webhook signatures using Stripe's signature verification. Never trust webhook data without verification.
  - **Idempotency**: Use Stripe's idempotency keys to prevent duplicate charges during retries.

### Component 8: Secrets Management
- **GCP Service**: Secret Manager
- **Developer Guidance**:
  - **Storing Secrets**: Use `gcloud` CLI or console to store secrets:
    ```bash
    echo -n "sk_live_xyz" | gcloud secrets create stripe-secret-key --data-file=-
    echo -n "SG.xyz" | gcloud secrets create sendgrid-api-key --data-file=-
    ```
  - **Accessing in Cloud Run**: Mount secrets as environment variables:
    ```bash
    gcloud run deploy compliancesync-api \
      --update-secrets STRIPE_SECRET_KEY=stripe-secret-key:latest \
      --update-secrets SENDGRID_API_KEY=sendgrid-api-key:latest
    ```
  - **Runtime Access**: Secrets appear as environment variables:
    ```javascript
    const stripe = require('stripe')(process.env.STRIPE_SECRET_KEY);
    ```
  - **Rotation**: Update secret versions in Secret Manager, redeploy Cloud Run with new version.
  - **IAM**: Grant Cloud Run service account `roles/secretmanager.secretAccessor` role.

### Component 9: Email Notifications
- **Integration**: SendGrid (or Mailgun) via Cloud Run
- **Developer Guidance**:
  - **SDK**: Use `@sendgrid/mail` (Node.js) or `sendgrid` (Python)
  - **Trigger**: Pub/Sub messages published to `notification-topic` trigger Cloud Run worker
  - **Code Example**:
    ```javascript
    const sgMail = require('@sendgrid/mail');
    sgMail.setApiKey(process.env.SENDGRID_API_KEY);

    app.post('/send-notification', async (req, res) => {
      const message = JSON.parse(Buffer.from(req.body.message.data, 'base64').toString());

      await sgMail.send({
        to: message.recipientEmail,
        from: 'notifications@compliancesync.com',
        subject: message.subject,
        html: message.htmlContent
      });

      res.status(200).send('OK');
    });
    ```
  - **Templates**: Use SendGrid dynamic templates for consistent branding. Pass template variables in API call.
  - **Tracking**: Enable open and click tracking in SendGrid. Store delivery status in Firestore if needed.

## 4. High-Level System Flow

### Example Flow 1: User Uploads Evidence Document

1. **User authenticates** via Identity Platform (web app), receives JWT token with `organizationId` custom claim
2. **User clicks "Upload Evidence"** in frontend; frontend calls Cloud Run API: `POST /api/evidence/upload-url`
3. **Cloud Run verifies JWT**, extracts `organizationId`, generates signed Cloud Storage upload URL for path `gs://bucket/{orgId}/evidence/{evidenceId}.pdf` (15-min expiration)
4. **Cloud Run returns signed URL** to frontend and creates pending evidence record in Firestore: `/organizations/{orgId}/evidence/{evidenceId}` with status "uploading"
5. **Frontend uploads file directly to Cloud Storage** using signed URL (bypasses backend for efficiency)
6. **Frontend notifies backend** of upload completion: `POST /api/evidence/{evidenceId}/complete`
7. **Cloud Run updates Firestore** evidence record with file metadata (size, content type) and status "active"
8. **Cloud Run writes audit log** to Firestore: `/organizations/{orgId}/evidence/{evidenceId}/auditLogs/{logId}` with action "created"
9. **Cloud Run publishes message to Pub/Sub** `notification-topic` to notify compliance admin
10. **Email worker (Cloud Run)** receives Pub/Sub message, sends email via SendGrid

### Example Flow 2: Background Gmail Polling for Evidence Capture

1. **Cloud Scheduler triggers** every 15 minutes, publishes message to Pub/Sub `gmail-poll-topic`
2. **Gmail worker (Cloud Run)** receives Pub/Sub push notification at `/process` endpoint
3. **Worker queries Firestore** for all organizations with Gmail integration enabled: `db.collection('organizations').where('integrations.gmail.enabled', '==', true).get()`
4. **For each organization**, worker retrieves encrypted OAuth refresh token from Firestore, decrypts using Cloud KMS
5. **Worker refreshes OAuth access token** using Google OAuth2 client library if expired
6. **Worker polls Gmail API** using refreshed token: `gmail.users.messages.list()` with query for compliance-related keywords and date filters (since last poll timestamp)
7. **For each matching email**, worker downloads attachments via Gmail API
8. **Worker uploads attachments to Cloud Storage** using signed URLs: `gs://bucket/{orgId}/evidence/gmail_{messageId}_{attachmentId}.pdf`
9. **Worker creates evidence records in Firestore**: `/organizations/{orgId}/evidence/{evidenceId}` with metadata (email subject, sender, date, source: "gmail")
10. **Worker updates last poll timestamp** in Firestore: `/organizations/{orgId}/integrations/gmail/lastPollTime`
11. **Worker acknowledges Pub/Sub message** by returning HTTP 200 status

### Example Flow 3: PDF Compliance Report Generation

1. **User clicks "Generate Report"** in frontend; frontend calls Cloud Run API: `POST /api/reports/generate`
2. **Cloud Run verifies JWT**, creates report job record in Firestore with status "pending"
3. **Cloud Run publishes message to Pub/Sub** `pdf-generation-topic` with payload: `{ organizationId, reportId, frameworkId, dateRange }`
4. **PDF worker (Cloud Run)** receives Pub/Sub push notification
5. **Worker queries Firestore** for all evidence documents matching report criteria: `/organizations/{orgId}/evidence` filtered by framework and date range
6. **Worker generates PDF** using Puppeteer (headless Chrome) or wkhtmltopdf library, rendering HTML template with evidence data
7. **Worker uploads PDF to Cloud Storage**: `gs://bucket/{orgId}/reports/{reportId}.pdf`
8. **Worker updates Firestore** report record with file path and status "completed"
9. **Worker publishes notification message** to Pub/Sub `notification-topic` to email report link to user
10. **Email worker sends email** with signed Cloud Storage URL (1-hour expiration) for report download

## 5. Architectural Principles & Considerations

### Scalability

This architecture scales automatically from 90 to 1000+ organizations without infrastructure changes:

- **Cloud Run auto-scaling**: Scales from 0 to max instances based on request rate. Each instance handles 80 concurrent requests. Configure `--max-instances` per service to control costs.
- **Firestore**: Automatically scales to handle millions of documents. Each organization's data is isolated in separate document trees, allowing parallel queries without contention.
- **Cloud Storage**: Supports unlimited file storage with consistent performance. Uses signed URLs to distribute download load directly to GCP edge network (no backend bottleneck).
- **Pub/Sub**: Handles message fan-out for background jobs. As organizations grow, workers process messages in parallel across Cloud Run instances.

**Bottleneck mitigation**:
- Rate limit Gmail/Drive API polling per organization (avoid quota exhaustion)
- Use Firestore batch operations for bulk writes (audit logs, evidence imports)
- Implement circuit breakers for external API calls (Stripe, SendGrid, OAuth providers)
- Monitor Cloud Run instance count and adjust concurrency settings if cold starts impact latency

**Cost-efficient scaling**: Pay-per-use model means costs scale linearly with actual usage, not pre-provisioned capacity. Example: Cloud Run charges only for CPU/memory during request processing (not idle time). Firestore charges per document read/write (not storage capacity).

### Security

**Authentication & Authorization**:
- Identity Platform provides secure JWT-based authentication with industry-standard OAuth 2.0 flows
- Custom claims in JWT tokens enforce tenant isolation (never trust client-provided `organizationId`)
- All Cloud Run endpoints verify JWT using Firebase Admin SDK before processing requests
- Use Workload Identity to grant Cloud Run services access to GCP resources (no service account keys in code)

**Data Isolation**:
- Firestore collection structure enforces tenant boundaries: `/organizations/{orgId}/...`
- Every database query filters by authenticated user's `organizationId` from JWT
- Firestore Security Rules provide defense-in-depth even if application logic fails
- Cloud Storage uses tenant-prefixed paths: `{orgId}/evidence/...` with IAM-based access control

**Encryption**:
- Data encrypted at rest by default (Firestore, Cloud Storage, Secret Manager)
- Data encrypted in transit via HTTPS/TLS 1.3 for all services
- OAuth tokens encrypted in Firestore using Cloud KMS envelope encryption before storage
- Stripe handles PCI-compliant card data; never store payment details in your infrastructure

**Secrets Management**:
- All API keys, OAuth credentials, and sensitive configuration stored in Secret Manager
- Secrets accessed via IAM roles (no keys committed to code repositories)
- Automatic secret rotation with versioning support

**Audit & Compliance**:
- Immutable audit logs in Firestore track all evidence access/modifications
- Cloud Logging captures system-level events (API access, authentication failures)
- Firestore Security Rules prevent audit log tampering
- Configure log exports to Cloud Storage for long-term retention (7+ years)

**Network Security**:
- Cloud Run services default to HTTPS-only endpoints
- Configure VPC connector if backend needs to access private resources (Cloud SQL, Redis)
- Use Cloud Armor (optional) for DDoS protection and WAF rules on public API endpoints
- Implement rate limiting in API Gateway to prevent abuse

### Cost Optimization

**Estimated Monthly Costs at 90 Organizations** (assuming 10 active users per org, 100 evidence uploads/org/month):

- Cloud Run (API + Workers): ~$50-100 (2M requests, minimal idle time)
- Firestore: ~$50-75 (500K document reads, 200K writes, 50GB storage)
- Cloud Storage: ~$10-20 (500GB evidence files, 10K operations)
- Identity Platform: Free tier (first 50K MAU free; ~900 users well within limit)
- Pub/Sub: ~$5 (minimal message volume for background jobs)
- Cloud Scheduler: ~$1 (10 jobs × $0.10/job)
- Secret Manager: ~$1 (10 secrets × $0.06/month)
- Cloud Logging: ~$10-20 (included free tier, overage charges minimal)
- **Total: ~$127-232/month**

**Cost Optimization Strategies**:

1. **Scale to Zero**: Configure Cloud Run `--min-instances 0` for all services except main API (consider `--min-instances 1` for API to reduce cold start latency during business hours)
2. **Firestore Index Optimization**: Minimize composite indexes (each index increases write costs). Use single-field indexes where possible.
3. **Cloud Storage Lifecycle Policies**: Move evidence files to Nearline storage class after 90 days (50% cost reduction) or Archive after 1 year (80% reduction) if access patterns permit.
4. **Efficient Queries**: Use Firestore query cursors for pagination (avoid fetching all documents at once). Implement caching in Cloud Run for frequently accessed data.
5. **Pub/Sub Acknowledgment**: Acknowledge messages quickly to avoid duplicate processing (increases costs)
6. **Monitoring & Alerts**: Set up budget alerts in Cloud Billing at $200, $500, and $1000 thresholds. Monitor per-service costs weekly.
7. **Reserved Capacity (Future)**: At 500+ organizations, consider committed use discounts for Cloud Run (25-50% savings)

**Cost Projection at 1000 Organizations**: Estimated ~$1,500-2,500/month (linear scaling). At $149-699/month per customer, this represents 5-15% COGS (gross margin: 85-95%).

### Operational Complexity

**Minimal Operations Required**:
- No servers to patch or maintain (all services fully managed)
- No database backups to manage (Firestore has automatic point-in-time recovery)
- No load balancer configuration (Cloud Run includes HTTPS load balancing)
- No certificate management (Cloud Run auto-provisions SSL/TLS)

**Monitoring & Observability**:
- **Cloud Monitoring**: Create dashboard tracking:
  - Cloud Run request latency (p50, p95, p99)
  - Error rates (5xx responses) per service
  - Firestore read/write latency
  - Pub/Sub message age (detect stuck jobs)
- **Cloud Logging**: Set up log-based metrics for:
  - Authentication failures (potential security issues)
  - Failed payment processing (revenue impact)
  - OAuth token refresh failures (integration issues)
- **Alerting**: Configure alerts for:
  - Error rate >1% for 5 minutes (PagerDuty/Slack)
  - Cloud Run 95th percentile latency >2s
  - Pub/Sub unacknowledged message age >30 minutes
  - Firestore quota exhaustion warnings
- **Uptime Checks**: Configure Cloud Monitoring uptime checks for Cloud Run API endpoints (alert on downtime)

**Deployment & CI/CD**:
- Use Cloud Build for automated deployments triggered by Git commits
- Implement blue-green deployments with Cloud Run traffic splitting (gradual rollout: 10% → 50% → 100%)
- Run integration tests against Firestore emulator locally before deployment
- Use Terraform or gcloud CLI scripts for infrastructure-as-code (optional but recommended)

**Disaster Recovery**:
- Firestore: Enable point-in-time recovery (PITR) for up to 7 days. Export database weekly to Cloud Storage for long-term backups.
- Cloud Storage: Enable object versioning and set retention policy to prevent accidental deletion
- Secrets: Secret Manager maintains version history automatically
- RTO/RPO: Recovery Time Objective <1 hour, Recovery Point Objective <24 hours (daily Firestore exports)

**Team Requirements**:
- Single full-stack engineer can manage this architecture initially
- On-call rotation recommended at 200+ organizations
- Minimal GCP expertise required (managed services abstract complexity)

## 6. Alternative Approaches Considered

**Cloud SQL vs Firestore**: Cloud SQL (PostgreSQL) was considered for relational integrity and SQL querying. Rejected because:
- Firestore better handles document-heavy compliance data (evidence metadata, audit logs) without rigid schemas
- No connection pooling complexity in serverless environment (Cloud SQL requires Cloud SQL Proxy or connection management)
- Firestore auto-scales without manual sharding or read replicas
- However, Cloud SQL may be reconsidered if complex multi-tenant reporting queries require SQL joins

**GKE vs Cloud Run**: Google Kubernetes Engine considered for container orchestration. Rejected because:
- GKE requires significant operational expertise (cluster management, node upgrades, autoscaling configuration)
- Cloud Run provides equivalent container execution with zero operational overhead
- Early-stage team benefits from managed simplicity over Kubernetes flexibility
- However, GKE may be appropriate if you need long-running stateful services or complex service mesh requirements

**Cloud Functions vs Cloud Run**: Cloud Functions (2nd gen) considered for background workers. Rejected because:
- Cloud Run allows code reuse (same codebase for API and workers)
- Longer execution timeout (60 min vs 9 min) critical for PDF generation
- Better local development experience (standard Docker containers)
- However, Cloud Functions may be simpler for single-purpose event handlers (e.g., Cloud Storage triggers)

**Cloud Tasks vs Pub/Sub**: Cloud Tasks considered for job queuing. Rejected because:
- Pub/Sub better handles fan-out patterns (one scheduler job triggers multiple organization polls)
- Pub/Sub provides topic-based decoupling (easier to add new worker types)
- Cloud Tasks better for targeted delivery to specific endpoints with retry logic; Pub/Sub better for broadcast messaging
- However, Cloud Tasks recommended if you need guaranteed exactly-once processing with precise scheduling

**BigQuery vs Firestore for Audit Logs**: BigQuery considered for analytics on audit trails. Rejected because:
- Firestore sufficient for per-tenant audit queries (no cross-tenant analytics needed initially)
- BigQuery more cost-effective at massive scale (TBs of logs), but overkill for 90-1000 organizations
- Streaming inserts to BigQuery add latency and cost
- However, export Firestore audit logs to BigQuery if customers request cross-organizational compliance analytics

**Auth0 vs Identity Platform**: Auth0 considered for authentication. Rejected because:
- Identity Platform (Firebase Auth) more cost-effective at your scale (free tier covers 900 MAU)
- Native GCP integration (no external dependency)
- Similar feature parity for OAuth and custom claims
- However, Auth0 provides richer enterprise features (adaptive MFA, breached password detection) if needed for compliance

## 7. Disclaimer

This architectural design represents a recommended starting point based on the provided requirements. It should be reviewed and refined by the engineering team based on:
- **Specific performance benchmarks and load testing results**: Validate Cloud Run concurrency settings and Firestore query performance under realistic load. Test PDF generation time with large reports.
- **Detailed security and compliance requirements**: Consult with compliance experts regarding specific regulations (SOC 2, HIPAA, GDPR). May require additional controls like VPC Service Controls or customer-managed encryption keys (CMEK).
- **Budget constraints and cost projections**: Monitor actual costs in first 3 months and adjust service configurations. Consider cost optimizations like Cloud CDN for file delivery or Memorystore (Redis) for caching if needed.
- **Team expertise and organizational standards**: Adapt technology choices to team strengths. If team has deep PostgreSQL experience, Cloud SQL may reduce development time despite operational complexity.
- **Integration requirements with existing systems**: If customers require on-premise integrations or private network access, consider Cloud VPN or Interconnect. If enterprise customers require SSO, implement SAML support via Identity Platform.

The final implementation may require adjustments as requirements evolve or new constraints are discovered during development. Key decision points to revisit:
1. **Month 3**: Evaluate actual costs vs projections; optimize expensive services
2. **Month 6**: Assess Firestore query performance at scale; consider Cloud SQL migration if complex reporting needed
3. **Month 12**: Review operational burden; consider GKE if Cloud Run limitations impact product roadmap

This ADR should be treated as a living document, updated as architectural decisions change based on real-world feedback.

---

## Next Steps for Development Team

1. Set up GCP project and enable required APIs (Cloud Run, Firestore, Identity Platform, Cloud Storage, Pub/Sub, Secret Manager)
2. Create Firestore database in `us-central1` (or region nearest to target customers)
3. Deploy skeleton Cloud Run API service with authentication middleware
4. Implement multi-tenant data model in Firestore with security rules
5. Build file upload flow with signed Cloud Storage URLs
6. Configure Cloud Scheduler + Pub/Sub for first background job (Gmail polling)
7. Set up monitoring dashboard and alerts in Cloud Monitoring
