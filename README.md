# CloudSecGuard — Infrastructure Security Audit Platform

![Version](https://img.shields.io/badge/version-0.1.0--beta-blue)
![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)

All-in-one self-hosted security audit platform for infrastructure, code, SSL/TLS and DNS.  
Run automated security scans, track findings, manage remediation, and deliver professional reports to clients.

---

## Table of Contents

- [What Is CloudSecGuard](#what-is-cloudsecguard)
- [Architecture](#architecture)
- [Quick Start (Docker)](#quick-start-docker)
- [Development Mode](#development-mode)
- [First Login](#first-login)
- [Navigation Guide](#navigation-guide)
  - [Overview — Dashboard](#overview--dashboard)
  - [Security — Cloud Audits](#security--cloud-audits)
  - [Security — Code and IaC](#security--code-and-iac)
  - [Security — Findings](#security--findings)
  - [Security — Remediation](#security--remediation)
  - [Security — Monitoring](#security--monitoring)
  - [Compliance — Frameworks](#compliance--frameworks)
  - [Compliance — Evidence](#compliance--evidence)
  - [Compliance — Policies](#compliance--policies)
  - [Compliance — Access Reviews](#compliance--access-reviews)
  - [Reports — Reports](#reports--reports)
  - [Reports — Audit Types](#reports--audit-types)
  - [Account — Plans](#account--plans)
  - [Account — Settings](#account--settings)
- [Connection Types](#connection-types)
- [Audit Job Lifecycle](#audit-job-lifecycle)
- [Share Links](#share-links)
- [Auditor Portal](#auditor-portal)
- [Plans and License](#plans-and-license)
- [CLI Audit Engine](#cli-audit-engine)
- [Environment Variables](#environment-variables)
- [Required Tools (CLI mode)](#required-tools-cli-mode)
- [Roadmap](#roadmap)

---

## What Is CloudSecGuard

CloudSecGuard has two operating modes:

| Mode | What it does |
|------|-------------|
| **Web Platform** | Browser-based dashboard — manage connections, run audits, track findings, generate reports, invite team members |
| **CLI Engine** | Headless scanner — run a full audit from the command line and get HTML/DOCX reports instantly |

**Supported scan types:**

| Type | Status | What gets scanned |
|------|--------|------------------|
| DigitalOcean | Stable | Droplets, firewalls, managed databases, App Platform, Spaces buckets, VPCs, load balancers |
| Code and IaC | Stable | Secrets (gitleaks), static analysis (semgrep), Terraform (trivy + custom DO rules), npm audit |
| SSL/TLS | Stable | Certificate validity, chain trust, protocol versions, cipher strength, HSTS, OCSP |
| DNS | Stable | SPF, DKIM, DMARC, DNSSEC, CAA records, zone transfer exposure |

---

## Architecture

```
CloudSecGuard/
├── cmd/
│   ├── infra-audit/     # CLI: DigitalOcean infrastructure scan
│   ├── code-audit/      # CLI: Code & Terraform scan
│   ├── html2docx/       # CLI: convert HTML report to DOCX
│   └── keygen/          # License key generator (internal)
├── internal/
│   ├── scanner/         # DO API scanner, Spaces scanner
│   ├── scanner/code/    # semgrep, gitleaks, trivy, npm, hclscan wrappers
│   ├── rules/           # Custom security rules registry
│   ├── report/          # HTML and DOCX report generators
│   ├── model/           # Shared data models
│   └── doapi/           # DigitalOcean API client
└── web/
    ├── backend/         # Go HTTP server (chi router, PostgreSQL, JWT auth)
    ├── frontend/        # React + TypeScript + TailwindCSS + shadcn/ui
    └── docker-compose.yml
```

The web backend exposes a REST API at `/api/*`. The React frontend is served via Nginx and proxies all API calls to the backend. PostgreSQL is the only external dependency.

---

## Quick Start (Docker)

Prerequisites: Docker and Docker Compose installed.

```bash
git clone git@github.com:Quixgit/CloudSecGuard.git
cd CloudSecGuard/web

# Copy and configure environment
cp .env.example .env
# Edit .env if you want Google OAuth (optional)

# Build and start all services
docker compose up -d --build

# Check logs
docker compose logs -f
```

| Service | URL |
|---------|-----|
| Web UI | http://localhost:3000 |
| Backend API | http://localhost:8080 |

On first start the platform creates an admin account. Check the backend logs for the initial credentials:

```bash
docker compose logs backend | grep -i "admin\|created\|password"
```

Change the password immediately after first login via **Settings -> Security**.

---

## Development Mode

**1. Start PostgreSQL:**
```bash
docker run -d --name csg-pg \
  -e POSTGRES_DB=infra_audit \
  -e POSTGRES_USER=<db-user> \
  -e POSTGRES_PASSWORD=<db-password> \
  -p 5432:5432 postgres:16-alpine
```

**2. Run the backend:**
```bash
cd CloudSecGuard

DATABASE_URL="postgres://<db-user>:<db-password>@localhost:5432/infra_audit?sslmode=disable" \
ASSETS_DIR="$(pwd)/assets" \
DATA_DIR="$(pwd)/web/data" \
go run ./web/backend/
```

**3. Run the frontend:**
```bash
cd web/frontend
npm install
npm run dev
# Opens at http://localhost:5173
# Vite automatically proxies /api to localhost:8080
```

---

## First Login

1. Open http://localhost:3000
2. Check the backend startup logs for the initial admin credentials
3. Log in and immediately go to **Settings -> Security** to set a strong password
4. Go to **Settings -> Profile** — fill in your name and organization details (these appear in every generated report)
5. Go to **Settings -> Branding** — upload your logo, watermark, and footer background image
6. Go to **Account -> Plans** — enter your license key to unlock additional features

---

## Navigation Guide

The sidebar is divided into four sections: **Security**, **Compliance**, **Reports**, **Account**.  
Each section can be independently enabled or disabled in **Settings -> Modules**.

Module status: `Stable` = production-ready / `Beta` = functional, actively improved / `Planned` = upcoming release

---

### Overview — Dashboard

**Path:** `/` — Status: Stable

The main command center. Auto-refreshes every 30 seconds.

**Top row — stat cards (clickable):**

| Card | What it shows |
|------|--------------|
| Connections | Total configured connections vs your plan limit |
| Audits this week | Number of audit jobs run in the last 7 days |
| Total findings | Open findings across all audits — highlighted when > 0 |

**Findings Trend chart** — Area chart of critical / high / medium / low findings over the last 30 days. Useful for spotting security regressions after deployments.

**Security Score** — Ring indicator showing the average score across all monitored connections. Score 0–100: below 60 is critical, 60–79 needs attention, 80 and above is healthy.

**Compliance Readiness** — Progress bars for each active compliance framework. Shows percentage of controls currently met.

**Remediation Progress** — Percentage of findings moved to Done on the Remediation board. Highlights overdue tasks.

**Security Policies** — Count of approved policies. Alerts on expired documents.

**Access Reviews** — Completion status. Alerts for reviews due this month.

**Recent Audits** — Last 5–10 jobs with status and finding counts. Click any row to open the full job detail with logs and downloads.

**New Audit** button (top-right) — navigates directly to Audit Types to start a scan.

---

### Security — Cloud Audits

**Path:** `/cloud-audits` — Status: Stable

Manage DigitalOcean infrastructure connections and run cloud security audits.

**Connection card shows:**
- Name and scope mode badge (project / hybrid / account)
- Masked API token
- Project ID and Spaces buckets if configured

**Actions per card:**

| Button | Action |
|--------|--------|
| Run Audit | Starts an audit job immediately, then redirects to live progress |
| History | Shows all past jobs for this connection with finding delta vs previous run |
| Schedule | Set daily or weekly recurring audits — requires Starter plan or above |
| Edit | Open Connection Form to modify settings |
| Delete | Removes the connection and all associated audit data |

**Bulk run:** Select multiple connections via checkboxes, then click **Run N selected** to start all simultaneously.

#### DigitalOcean Connection Settings

| Field | Required | Description |
|-------|----------|-------------|
| Name | Yes | Human-readable label, e.g. "Production EU" |
| DO API Token | Yes | DigitalOcean personal access token with read scope |
| Project ID | No | UUID from DO Dashboard -> Projects. Omit for full account scan |
| Scope mode | Yes | See table below |
| Spaces buckets | No | Format: `bucket:region:sensitivity` — comma-separated |

**Scope modes:**

| Mode | What gets scanned |
|------|------------------|
| `project` | Only resources explicitly assigned to the DO project |
| `hybrid` | Project resources plus resources inferred to belong to it (recommended) |
| `account` | All resources across the entire DO account |

**Test connection** — validates the token against the DO API and lists accessible projects before saving.

---

### Security — Code and IaC

**Path:** `/code-iac` — Status: Stable — requires Professional plan

Manage source code repository connections and run security scans.

Same card and actions interface as Cloud Audits. Connection type is Code.

#### Code Connection Settings

| Field | Required | Description |
|-------|----------|-------------|
| Name | Yes | Label for this repo or project |
| Source | Yes | `git` — clone from URL, or `local` — path on the server |
| Repository URL | git | Full HTTPS or SSH URL |
| Branch | git | Branch to scan, default: main |
| Access token | git | GitHub or GitLab token for private repos |
| Local path | local | Absolute server path where the repo is already cloned |

**What gets scanned — auto-detected by stack:**

| Stack | Tools |
|-------|-------|
| All repositories | gitleaks — hardcoded secrets, credentials, API keys |
| Node.js / TypeScript | semgrep rules + npm audit for known CVEs |
| Docker | semgrep Dockerfile security rules |
| GitHub Actions / GitLab CI | semgrep CI/CD pipeline rules |
| Terraform | trivy config + hclscan (DigitalOcean-specific rules) |

---

### Security — Findings

**Path:** `/findings` — Status: Stable

Aggregated view of all findings across every audit job.

**Top cards:** Critical, High, Medium, Low — counts with color indicators.

**Filters:** Severity / Connection / Status (Open, Acknowledged, Resolved)

**Findings table:**

| Column | Description |
|--------|-------------|
| Severity | Color-coded badge |
| Title | Finding name, e.g. "Firewall allows unrestricted inbound SSH" |
| Resource | Affected droplet, database, file path, or domain |
| Connection | Which connection the finding came from |
| First seen | Date of first detection |
| Status | Open / Acknowledged / Resolved |

Click a row to expand full details: description, technical evidence, and step-by-step remediation guidance.

---

### Security — Remediation

**Path:** `/remediation` — Status: Beta

Kanban board for tracking the remediation lifecycle of security findings.

**Board columns:**

| Column | Meaning |
|--------|---------|
| Backlog | Newly created tasks, not yet prioritized |
| In Progress | Actively being worked on |
| Review | Fix deployed, pending verification |
| Done | Fully resolved and confirmed |

**Task card:** Severity label, title, due date (highlighted if overdue), assignee.

**Current capabilities:**
- Create and manage remediation tasks
- Set due date and assignee per task
- Move tasks between columns
- Dashboard widget shows overall progress percentage

**Planned:**
- Auto-import findings directly from completed audit jobs
- Link tasks to specific finding records
- Email and Slack notifications for overdue tasks
- Bulk assign and priority sort
- Jira and Linear integration

---

### Security — Monitoring

**Path:** `/monitoring` — Status: Beta

Continuous security posture tracking and SLA monitoring.

**Security Score per connection** — score 0–100 based on finding severity and count.

**SLA tracking** — define maximum time-to-remediate per severity level (e.g. Critical = 7 days). Breach indicator appears when a finding exceeds its deadline.

**Current capabilities:**
- Per-connection security score
- SLA breach count and indicator
- Dashboard widget with average score across all connections

**Planned:**
- Historical score trend chart per connection
- Automated alerts when score drops below a defined threshold
- SLA policy configuration per team or connection group
- Weekly digest email report
- Webhook push on score change

---

### Compliance — Frameworks

**Path:** `/compliance` — Status: Beta

Map audit findings to industry compliance frameworks and track readiness.

**Supported frameworks:**

| Framework | Description |
|-----------|-------------|
| SOC 2 | Trust Service Criteria — security, availability, confidentiality |
| ISO 27001 | Information Security Management System controls |
| NIST CSF | Cybersecurity Framework — Identify, Protect, Detect, Respond, Recover |
| CIS v8 | Center for Internet Security Controls v8 |

**Per framework view:**
- Controls breakdown: Met / Partially met / Not met
- Score percentage with progress bar
- Controls can be linked to evidence artifacts

**How controls get marked as met:**
- Automatically when a related audit finding is set to Resolved
- Manually by linking an evidence artifact to the control

**Planned:**
- Automatic mapping of findings to controls by rule ID
- Gap analysis report export
- SOC 2 Type II readiness calendar view
- PCI-DSS and HIPAA frameworks
- Auditor questionnaire generation

---

### Compliance — Evidence

**Path:** `/evidence` — Status: Beta

Store and manage evidence artifacts for compliance audits.

**Evidence types:**
- Audit reports auto-generated by CloudSecGuard jobs
- Screenshots and documents (PDF or PNG)
- Policy documents and approval records
- Meeting minutes, sign-offs, configuration exports

**Each evidence item has:** Title, type, description, linked compliance control, uploaded file or URL, review date, and owner.

**Current capabilities:** Create, upload, and list evidence items with control linking.

**Planned:**
- Bulk upload via drag and drop
- Evidence request workflow (request -> submit -> approve)
- Auto-attach completed audit reports as evidence
- Expiry notifications
- Auditor read-only evidence portal

---

### Compliance — Policies

**Path:** `/policies` — Status: Beta

Write, approve, and version-control internal security policies.

**Policy fields:**
- Title and category (Access Control, Data Protection, Incident Response, Acceptable Use, etc.)
- Content editor
- Owner, reviewer, and approval status
- Review cycle (annual, semi-annual, quarterly)
- Effective and expiry dates

**Policy lifecycle:** Draft -> Review -> Approved -> Expired

**Dashboard widget** shows approved count and flags expired policies.

**Planned:**
- Version history and diff view
- Approval workflow with sign-off tracking
- Policy acknowledgement tracking per team member
- Automatic expiry reminders to policy owners
- Starter template library (ISO 27001, SOC 2)
- PDF export with company branding

---

### Compliance — Access Reviews

**Path:** `/access-reviews` — Status: Beta

Periodic reviews of who has access to which systems — a core requirement for SOC 2 and ISO 27001.

**Review fields:**
- System or resource name
- Review owner and due date
- Access list: each entry has user, role, justification, and decision (approve or revoke)

**Review lifecycle:** Pending -> In Progress -> Completed

**Dashboard widget** shows overdue reviews and those due this month.

**Planned:**
- Auto-import users from connected DigitalOcean teams
- Integration with Okta, Google Workspace, GitHub Org members
- Automated reviewer email reminders
- Bulk certify or revoke access
- Export completed reviews as audit evidence
- Quarterly campaign scheduling

---

### Reports — Reports

**Path:** `/reports` — Status: Stable

Complete audit history with download links for all finished jobs.

**Table columns:**

| Column | Description |
|--------|-------------|
| Connection | Which system was audited |
| Type | Cloud / Code / SSL / DNS |
| Status | queued / running / done / failed |
| Started | Timestamp |
| Findings | C / H / M / L breakdown (done jobs only) |
| Actions | Download HTML, Download DOCX, Share link |

**Report formats:**

| Format | Best for |
|--------|---------|
| HTML | Browser preview, print to PDF, internal review |
| DOCX | Client delivery, editing in Word or Google Docs |

**DigitalOcean report sections:**
1. Executive Summary
2. Scope and Methodology
3. Risk Matrix
4. Findings — per resource with severity, evidence, and remediation steps
5. Infrastructure Inventory — droplets, databases, apps, Spaces
6. Appendix — raw JSON evidence

**Code security report sections:**
1. Executive Summary
2. Secret Exposure Findings
3. Static Analysis Findings — per file and line number
4. Dependency Vulnerabilities — npm audit CVEs
5. Terraform / IaC Misconfigurations
6. Appendix

---

### Reports — Audit Types

**Path:** `/audit-types` — Status: Stable

Starting point for launching any new audit. Four type cards are shown:

| Card | What it scans | Plan required |
|------|--------------|---------------|
| DigitalOcean | Cloud infrastructure | Free |
| Code and IaC | Repository and Terraform | Professional |
| SSL/TLS | Certificate and TLS configuration | Free |
| DNS | DNS record security | Free |

Clicking a card navigates to the corresponding connections page. If no connection exists yet, **New Connection** opens the Connection Form for that type.

---

### Account — Plans

**Path:** `/plans` — Status: Stable

View plan tiers and manage your license key.

**Plan tiers:**

| Plan | Price | Connections | Audits/mo | Users | Key features |
|------|-------|-------------|-----------|-------|-------------|
| Free | $0 | 5 | 20 | 1 | Basic audits, share links |
| Starter | $19/mo | 10 | 30 | 2 | + Scheduled audits, PDF reports |
| Pro | $49/mo | 30 | 100 | 5 | + Code audits, custom branding |
| Business | $99/mo | Unlimited | Unlimited | 15 | + API tokens, team management |
| Enterprise | Custom | Unlimited | Unlimited | Unlimited | + All features, priority support |

**Activating a license:**
1. Obtain a license key from your account portal
2. Open **Account -> Plans** and scroll to the License Key section
3. Paste the key and click **Activate**

The license is verified cryptographically (RSA signature) and takes effect immediately without a restart.

**Admin preview plan** (Settings -> Team):
Admins and owners can preview any plan's features locally for testing. Does not affect regular users.

---

### Account — Settings

**Path:** `/settings` — Status: Stable

Seven-tab vertical sidebar covering all account and workspace configuration.

---

#### Settings — Profile

Personal information embedded in every generated report.

| Field | Description |
|-------|-------------|
| Auditor name | Shown as "Prepared by" on every report |
| Organization | Company name in report header |
| Email | Auditor contact in report footer |
| Phone | Auditor phone in report footer |
| Website | Auditor website in report footer |
| Address | Postal address for formal reports |
| Email notifications | Toggle: receive email when an audit job completes |

---

#### Settings — Security

Account security controls.

**Change password:** Enter current password, new password (minimum 8 characters), confirm, then click **Change password**.

**Two-Factor Authentication:**
1. Click **Set up 2FA**
2. Scan the QR code with any TOTP app (Google Authenticator, Authy, 1Password, etc.)
3. Enter the 6-digit code to confirm enrollment
4. 2FA is required at every subsequent login

To disable: Click **Disable 2FA** and confirm with a current TOTP code.

**API Tokens** — Business and Enterprise plans only:
- Generate named tokens for programmatic API access
- Token value is shown once at creation — store it securely
- Click Revoke next to any token to invalidate it immediately

---

#### Settings — Branding

Custom visual assets embedded in every generated report.

| Asset | Recommended format | Used in |
|-------|--------------------|---------|
| Logo | PNG with transparent background | Report cover page and header |
| Watermark | PNG, semi-transparent | Full-page background of every report page |
| Footer background | PNG, approximately 1920x200 px | Footer band across every report page |

If no custom file is uploaded, the default platform assets are used as fallback.

---

#### Settings — Team

Manage workspace members. Requires Business plan or above.

**Invite a member:**
1. Enter the person's email address
2. Select a role
3. Click **Send invite** — the person receives a one-time join link by email

**Roles:**

| Role | What they can do |
|------|----------------|
| Viewer | Read reports and findings — no write access |
| Auditor | Run audits, manage connections, download reports |
| Admin | Everything plus manage team members and workspace settings |
| Owner | Everything plus billing and license management |

**License section:** Shows current plan, limits, and expiry date. Input to enter or update a license key. Admin preview plan dropdown for testing features locally.

---

#### Settings — External

Create access portals for external clients and auditors.

**Auditor invites:**
- Generate a portal link scoped to a specific connection
- The client opens `/auditor/:token` — no account needed
- Configure permissions: view findings only, or full report with DOCX download

---

#### Settings — Modules

Enable or disable sidebar sections to match your team's workflow.

| Toggle | Controls |
|--------|---------|
| Cloud Audits | Security -> Cloud Audits |
| Code and IaC | Security -> Code and IaC |
| Findings | Security -> Findings |
| Remediation | Security -> Remediation |
| Monitoring | Security -> Monitoring |
| Compliance | Compliance -> Frameworks |
| Evidence | Compliance -> Evidence |
| Policies | Compliance -> Policies |
| Access Reviews | Compliance -> Access Reviews |
| Reports | Reports -> Reports |
| Audit Types | Reports -> Audit Types |

Changes take effect instantly — no reload required.

---

#### Settings — Activity

Workspace event audit log. Timestamped entries for:
- User logins and logouts
- Connection created, edited, or deleted
- Audit jobs started and completed
- Settings changed
- Team members invited or removed
- License key activated or changed

Used for security incident investigation and compliance audit trails.

---

## Connection Types

| Type | Input | What is checked | Output |
|------|-------|----------------|--------|
| DigitalOcean | DO API token + project | Firewall rules, open ports, database exposure, Spaces ACLs, App Platform secrets, TLS config | HTML + DOCX infrastructure report |
| Code | Git URL or local path | Hardcoded secrets, insecure patterns, Dockerfile hygiene, Terraform misconfigs, vulnerable npm packages | HTML + DOCX code security report |
| SSL/TLS | Domain(s) | Cert expiry, chain validity, deprecated TLS 1.0/1.1, weak ciphers, HSTS, OCSP stapling | Findings in platform |
| DNS | Domain(s) | SPF policy, DKIM presence, DMARC enforcement, DNSSEC, CAA record, zone transfer exposure | Findings in platform |

---

## Audit Job Lifecycle

```
New Connection
      |
      v
  Run Audit -----------------------------------------------+
      |                                                     |
      v                                                     v
  queued -> running -> done                             failed
                |                                          |
                v                                          v
         Findings saved to DB              Error captured (view in job detail)
                |
                v
        Reports generated
       (HTML + DOCX files)
                |
          +-----+------+
          v            v
       Download     Share link
      (Reports)    (public URL)
```

**Job Detail page** (`/jobs/:id`):
- Real-time progress bar via WebSocket
- Live scanner log output
- Findings table once the job completes
- Download buttons for HTML and DOCX
- Share button to generate a read-only public link

---

## Share Links

A share link lets you send an audit report to a client without requiring an account.

**To create a share link:**
1. Open a finished job or go to Reports
2. Click **Share**
3. Copy the generated URL — format: `/share/:token`

**What the recipient sees:**
- Read-only report with full findings, severity breakdown, and remediation guidance
- No login required
- Token is unique per job

---

## Auditor Portal

A white-labeled, read-only client-facing view.

**How it works:**
1. Admin generates an invite link in **Settings -> External**
2. Send the link to the client: `https://your-domain.com/auditor/<token>`
3. Client opens it — sees findings, severity breakdown, and report download button
4. No account creation needed
5. Access is scoped to the specific connection only

---

## Plans and License

Plan limits are enforced at the API level.

| Limit | Free | Starter | Pro | Business | Enterprise |
|-------|------|---------|-----|----------|------------|
| Connections | 5 | 10 | 30 | Unlimited | Unlimited |
| Audits/month | 20 | 30 | 100 | Unlimited | Unlimited |
| Users | 1 | 2 | 5 | 15 | Unlimited |
| Scheduled audits | No | Yes | Yes | Yes | Yes |
| Code audits | No | No | Yes | Yes | Yes |
| Share links | Yes | Yes | Yes | Yes | Yes |
| Custom branding | No | No | Yes | Yes | Yes |
| API tokens | No | No | No | Yes | Yes |
| Team management | No | No | No | Yes | Yes |

When a gated feature is accessed on an insufficient plan, the UI shows a lock icon and a prompt to upgrade.

---

## CLI Audit Engine

Run audits directly from the server without the web UI.

### Prerequisites

```bash
# gitleaks — secret scanning
go install github.com/gitleaks/gitleaks/v8@latest

# semgrep — static analysis
pip install semgrep

# trivy — IaC / container scanning
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin

# npm — Node.js dependency audit (bundled with Node.js)
node --version && npm --version

# Verify everything is present
which gitleaks semgrep trivy npm
```

---

### DigitalOcean Infrastructure Scan

```bash
export DO_TOKEN="<your-digitalocean-api-token>"

cd CloudSecGuard && go run ./cmd/infra-audit all-do \
  --client          "Client Company Name" \
  --project         "Client Company Infrastructure Audit" \
  --do-project-id   "<project-uuid-from-digitalocean>" \
  --scope-mode      hybrid \
  --spaces-buckets  "bucket-name:nyc3:sensitive" \
  --prepared-by     "Your Name / Your Team" \
  --auditor-org     "Your Company" \
  --auditor-email   "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone   "+1 555 000 0000" \
  --auditor-address $'123 Main St\nCity, State 00000' \
  --classification  "Confidential" \
  --period          "May 2026 point-in-time assessment" \
  --logo            assets/logo.png \
  --watermark       assets/watermark.png \
  --footer-bg       assets/footer-bg.png \
  --out             out/client-name
```

**Output structure:**
```
out/client-name/
  evidence/do_inventory.json        # Raw collected data
  report/
    client_project.html             # HTML report
    client_project.docx             # DOCX report
    client_project_findings.json    # Structured findings list
```

**`--scope-mode` options:**

| Value | What is scanned |
|-------|----------------|
| `project` | Only resources assigned to the DO project |
| `hybrid` | Project resources plus inferred related resources (recommended) |
| `account` | All resources across the entire DO account |

**`--spaces-buckets` format:**
`bucket-name:region:sensitivity` — comma-separated for multiple buckets.
Sensitivity values: `public` / `sensitive` / `internal`

---

### Code and IaC Scan

```bash
# Clone the target repository if not already local
git clone https://github.com/client/repo.git /tmp/client-repo

# Run the scanner
cd CloudSecGuard && go run ./cmd/code-audit \
  --repo            /tmp/client-repo \
  --client          "Client Company Name" \
  --project         "Client Company Code Security Audit" \
  --prepared-by     "Your Name / Your Team" \
  --auditor-org     "Your Company" \
  --auditor-email   "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone   "+1 555 000 0000" \
  --auditor-address $'123 Main St\nCity, State 00000' \
  --classification  "Confidential" \
  --period          "May 2026 point-in-time assessment" \
  --logo            assets/logo.png \
  --watermark       assets/watermark.png \
  --footer-bg       assets/footer-bg.png \
  --out             out/client-name-code
```

**Output:**
```
out/client-name-code/
  report/
    client_project.html    # HTML report
    client_project.docx    # DOCX report
```

---

### Full Audit — Both Reports

```bash
export DO_TOKEN="<your-digitalocean-api-token>"

CLIENT="Acme Corp"
DO_PROJECT_ID="<project-uuid-from-digitalocean>"
REPO_PATH="/tmp/acme-repo"
OUT="out/acme-$(date +%Y-%m)"

cd CloudSecGuard

# 1. Infrastructure
go run ./cmd/infra-audit all-do \
  --client "$CLIENT" \
  --project "$CLIENT Infrastructure Audit" \
  --do-project-id "$DO_PROJECT_ID" \
  --scope-mode hybrid \
  --prepared-by "Your Name / Your Team" \
  --auditor-org "Your Company" \
  --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone "+1 555 000 0000" \
  --auditor-address $'123 Main St\nCity, State 00000' \
  --classification "Confidential" \
  --period "$(date '+%B %Y') point-in-time assessment" \
  --logo assets/logo.png --watermark assets/watermark.png --footer-bg assets/footer-bg.png \
  --out "$OUT/infra"

# 2. Code
go run ./cmd/code-audit \
  --repo "$REPO_PATH" \
  --client "$CLIENT" \
  --project "$CLIENT Code Security Audit" \
  --prepared-by "Your Name / Your Team" \
  --auditor-org "Your Company" \
  --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone "+1 555 000 0000" \
  --auditor-address $'123 Main St\nCity, State 00000' \
  --classification "Confidential" \
  --period "$(date '+%B %Y') point-in-time assessment" \
  --logo assets/logo.png --watermark assets/watermark.png --footer-bg assets/footer-bg.png \
  --out "$OUT/code"

echo "Done. Reports:"
ls "$OUT/infra/report/" "$OUT/code/report/"
```

**Deliverable files:**

| File | Contents |
|------|---------|
| `*_infrastructure_audit.docx` | DO infrastructure: firewall, databases, apps, Spaces |
| `*_code_security_audit.docx` | Code: secrets, Dockerfile, GitHub Actions, Terraform |

HTML versions are for internal review and print-to-PDF.

---

## Environment Variables

**`web/.env` — optional, for Google OAuth:**

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth app client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth app client secret |
| `GOOGLE_REDIRECT_URL` | Redirect URI registered in Google Console |

Copy `web/.env.example` to `web/.env` and fill in values. Google OAuth is optional — email/password login works without it.

**Backend environment (via docker-compose or shell):**

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string (required) |
| `ASSETS_DIR` | `./assets` | Path to default logo / watermark / footer-bg |
| `DATA_DIR` | `./data` | File storage for uploaded user assets |
| `JWT_SECRET` | auto | Secret key for signing access tokens |
| `PORT` | `8080` | Backend listen port |

---

## Required Tools (CLI mode)

Only needed when running CLI audit commands directly. Not required for the Docker Compose web deployment.

| Tool | Purpose | Install |
|------|---------|---------|
| `gitleaks` | Hardcoded secret scanning | `go install github.com/gitleaks/gitleaks/v8@latest` |
| `semgrep` | Static code analysis | `pip install semgrep` |
| `trivy` | Terraform / IaC misconfiguration scanning | https://aquasecurity.github.io/trivy/latest/getting-started/installation |
| `npm` | Node.js dependency audit | Bundled with Node.js — https://nodejs.org |

---

## Roadmap

Features planned for upcoming releases.

### v0.2.0 — Modular Architecture
- Micro-frontend module federation (each scan type as an independent deployable unit)
- Argo CD ApplicationSet deployment per module
- Per-module Helm charts
- Module marketplace — enable or disable without full redeployment

### v0.2.x — Integrations
- Slack notifications for findings, job completions, and SLA breaches
- Jira and Linear issue creation directly from findings
- PagerDuty alerts for critical findings
- Webhook push events for CI/CD pipelines
- Native GitHub Actions and GitLab CI integration

### v0.3.0 — Cloud Providers
- AWS audit (EC2 security groups, S3 ACLs, IAM, CloudTrail)
- GCP audit (Compute, GCS, IAM, VPC)
- Azure audit (VMs, Blob, RBAC, NSGs)
- Kubernetes cluster audit (RBAC, Pod Security, Network Policies)

### v0.3.x — Advanced Scanning
- Container image scanning via trivy
- SBOM generation (CycloneDX and SPDX formats)
- GitHub and GitLab organization-wide secret scanning
- Dependency license compliance checks
- Custom rule builder in YAML

### v0.4.0 — Enterprise
- SAML 2.0 and OIDC SSO (Okta, Azure AD, Google Workspace)
- Role-based access scoped per connection, not just per workspace
- Multi-tenant isolation improvements
- Audit log export for SIEM integration
- Self-service compliance report generation (SOC 2 Type II readiness package)

---

## Support

- Issues and bug reports: https://github.com/Quixgit/CloudSecGuard/issues
