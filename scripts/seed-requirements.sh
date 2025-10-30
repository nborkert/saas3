#!/bin/bash

# ComplianceSync - Seed Regulatory Requirement Templates
# This script seeds Firestore with pre-built regulatory requirement templates

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo "========================================="
echo "Seed Regulatory Requirement Templates"
echo "========================================="
echo ""

read -p "Enter GCP Project ID: " PROJECT_ID

log_info "Setting project to $PROJECT_ID..."
gcloud config set project "$PROJECT_ID"

log_info "Creating requirement templates in Firestore..."

# Note: This is a placeholder script. In production, you would either:
# 1. Use the Firestore API or client library to seed data
# 2. Import from a JSON file using gcloud firestore import
# 3. Create an admin endpoint in the API to seed templates

log_warn "Template seeding requires implementation using one of these methods:"
echo "  1. Create a Go script using the Firestore client library"
echo "  2. Use 'gcloud firestore import' with JSON data"
echo "  3. Create an API endpoint at POST /api/v1/admin/seed-templates"
echo ""

cat > /tmp/requirement-templates.json <<'EOF'
{
  "templates": [
    {
      "id": "sec-ria-001",
      "framework": "SEC RIA",
      "title": "Code of Ethics",
      "description": "Establish and maintain a written code of ethics that addresses key aspects of fiduciary duty",
      "frequency": "annual",
      "category": "governance",
      "severity": "critical"
    },
    {
      "id": "sec-ria-002",
      "framework": "SEC RIA",
      "title": "Annual Compliance Review",
      "description": "Conduct annual review of compliance program effectiveness",
      "frequency": "annual",
      "category": "compliance",
      "severity": "critical"
    },
    {
      "id": "hipaa-001",
      "framework": "HIPAA",
      "title": "Security Risk Assessment",
      "description": "Conduct periodic security risk assessments to identify threats and vulnerabilities",
      "frequency": "annual",
      "category": "security",
      "severity": "critical"
    },
    {
      "id": "soc2-cc1.1",
      "framework": "SOC 2",
      "title": "Control Environment",
      "description": "The entity demonstrates a commitment to integrity and ethical values",
      "frequency": "continuous",
      "category": "governance",
      "severity": "high"
    },
    {
      "id": "iso27001-a5.1",
      "framework": "ISO 27001",
      "title": "Information Security Policies",
      "description": "A set of policies for information security shall be defined, approved by management, published and communicated to employees and relevant external parties",
      "frequency": "annual",
      "category": "policy",
      "severity": "critical"
    }
  ]
}
EOF

log_info "Sample templates saved to /tmp/requirement-templates.json"
echo ""
log_info "To seed these templates, use one of these methods:"
echo ""
echo "Method 1 - Using the API (recommended):"
echo "  1. Deploy the application"
echo "  2. Create an admin API endpoint to import templates"
echo "  3. POST the JSON file to that endpoint"
echo ""
echo "Method 2 - Using a Go script:"
echo "  1. Create a script in scripts/seed-data.go"
echo "  2. Use the Firestore client library to write templates"
echo "  3. Run: go run scripts/seed-data.go"
echo ""
echo "Method 3 - Manual via Firebase Console:"
echo "  1. Go to Firebase Console > Firestore Database"
echo "  2. Create collection: requirement_templates"
echo "  3. Add documents manually with the data from the JSON file"
echo ""

cat /tmp/requirement-templates.json
