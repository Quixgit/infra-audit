# CloudSecGuard — Infrastructure Security Audit Platform

<div align="center">

![Version](https://img.shields.io/badge/version-0.1.0--beta-blue)
![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)
![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)
![License](https://img.shields.io/badge/license-proprietary-red)

**All-in-one security audit platform for infrastructure, code, SSL/TLS and DNS.**  
Run automated security scans, track findings, manage remediation, and deliver professional reports to clients.

</div>

---

## Table of Contents

- [What Is CloudSecGuard](#what-is-cloudsecguard)
- [Architecture](#architecture)
- [Quick Start (Docker)](#quick-start-docker)
- [Development Mode](#development-mode)
- [First Login](#first-login)
- [Navigation Guide](#navigation-guide)
  - [Overview — Dashboard](#overview--dashboard)
  - [Security → Cloud Audits](#security--cloud-audits)
  - [Security → Code & IaC](#security--code--iac)
  - [Security → Findings](#security--findings)
  - [Security → Remediation](#security--remediation)
  - [Security → Monitoring](#security--monitoring)
  - [Compliance → Frameworks](#compliance--frameworks)
  - [Compliance → Evidence](#compliance--evidence)
  - [Compliance → Policies](#compliance--policies)
  - [Compliance → Access Reviews](#compliance--access-reviews)
  - [Reports → Reports](#reports--reports)
  - [Reports → Audit Types](#reports--audit-types)
  - [Account → Plans](#account--plans)
  - [Account → Settings](#account--settings)
- [Connection Types](#connection-types)
- [Audit Job Lifecycle](#audit-job-lifecycle)
- [Share Links](#share-links)
- [Auditor Portal](#auditor-portal)
- [Plans & License](#plans--license)
- [CLI Audit Engine](#cli-audit-engine)
- [Environment Variables](#environment-variables)
- [Required Tools (CLI mode)](#required-tools-cli-mode)

---

## What Is CloudSecGuard

CloudSecGuard is a self-hosted security audit platform with two operating modes:

| Mode | What it does |
|------|-------------|
| **Web Platform** | Browser-based dashboard — manage connections, run audits, track findings, generate reports, invite team members |
| **CLI Engine** | Headless scanner — run a full audit from the command line and get HTML/DOCX reports instantly |

**Supported scan types:**

| Type | What gets scanned |
|------|------------------|
| ☁️ **DigitalOcean** | Droplets, firewalls, managed databases, App Platform, Spaces buckets, VPCs, load balancers |
| 💻 **Code & IaC** | Secrets (gitleaks), static analysis (semgrep), Terraform (trivy + custom DO rules), npm audit |
| 🔒 **SSL/TLS** | Certificate validity, chain trust, protocol versions, cipher strength, HSTS, OCSP |
| 🌐 **DNS** | SPF, DKIM, DMARC, DNSSEC, CAA records, zone transfer exposure |

---

## Architecture

```
infra-audit/
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

**Prerequisites:** Docker + Docker Compose installed.

```bash
git clone git@github.com:Quixgit/CloudSecGuard.git
cd CloudSecGuard/web

# Copy and fill in the environment file
cp .env.example .env
# Edit .env — optional: add Google OAuth credentials

# Build and start
docker compose up -d --build

# View logs
docker compose logs -f
```

| Service | URL |
|---------|-----|
| Web UI | http://localhost:3000 |
| Backend API | http://localhost:8080 |

**Default credentials:**

| Field | Value |
|-------|-------|
| Email | `admin@infrajump.com` |
| Password | `InfraJump2026!` |

> ⚠️ Change the default password immediately after first login via **Settings → Security**.

---

## Development Mode

**Backend:**
```bash
cd CloudSecGuard

# Start PostgreSQL
docker run -d --name csg-pg \
  -e POSTGRES_DB=infra_audit \
  -e POSTGRES_USER=audit \
  -e POSTGRES_PASSWORD=audit123 \
  -p 5432:5432 postgres:16-alpine

# Run backend
DATABASE_URL="postgres://audit:audit123@localhost:5432/infra_audit?sslmode=disable" \
ASSETS_DIR="$(pwd)/assets" \
DATA_DIR="$(pwd)/web/data" \
go run ./web/backend/
```

**Frontend:**
```bash
cd web/frontend
npm install
npm run dev
# → http://localhost:5173 (Vite proxies /api to localhost:8080)
```

---

## First Login

1. Open http://localhost:3000
2. Log in with `admin@infrajump.com` / `InfraJump2026!`
3. Go to **Settings → Profile** — fill in your name and auditor organization details (these appear in every report)
4. Go to **Settings → Branding** — upload your logo, watermark, and footer background
5. Go to **Account → Plans** — enter your license key if you have one (unlocks scheduled audits, code scanning, team features)

---

## Navigation Guide

The sidebar has four sections: **Security**, **Compliance**, **Reports**, **Account**.

---

### Overview — Dashboard

**Path:** `/`

The main command center. Refreshes automatically every 30 seconds.

**Top row — stat cards (clickable):**

| Card | What it shows |
|------|--------------|
| **Connections** | Total configured connections. Shows your plan's limit |
| **Audits this week** | Number of audits run in the last 7 days |
| **Total findings** | Open findings across all audits. Orange if > 0 |

**Findings Trend chart** — Area chart showing critical/high/medium/low findings over the last 30 days. Helps spot spikes after deployments.

**Security Score** — Ring indicator showing the average security score across all monitored connections. Red < 60, yellow 60–79, green ≥ 80.

**Compliance Readiness** — Progress bars for each active compliance framework (SOC 2, ISO 27001, NIST CSF, CIS v8). Shows percentage of controls met.

**Remediation Progress** — Shows how many findings have been moved to Done on the Remediation board. Flags overdue tasks in red.

**Security Policies** — Count of approved/total policies. Red badge if any are expired.

**Access Reviews** — Count of completed/in-progress reviews. Yellow badge for reviews due this month.

**Recent Audits** — Last 5–10 audit jobs with status badges and finding counts. Click any row to open the full job detail.

**Quick action:** The **New Audit** button in the top-right navigates to Audit Types to start a scan immediately.

---

### Security → Cloud Audits

**Path:** `/cloud-audits`

Manage DigitalOcean infrastructure connections and run cloud audits.

**Connection card shows:**
- Connection name and scope badge (project / hybrid / account)
- Masked DO token
- Project ID (if scoped)
- Spaces buckets (if configured)

**Actions per connection card:**

| Button | Action |
|--------|--------|
| **Run Audit** | Starts an audit job immediately → redirects to live job progress |
| **History** (chart icon) | Opens a dialog with all past audit jobs for this connection, with finding delta vs previous run |
| **Schedule** (clock icon) | Set up recurring audits — daily or weekly. Requires Starter plan or above |
| **Edit** (pencil icon) | Opens the Connection Form to change settings |
| **Delete** (trash icon) | Deletes the connection and all associated audit jobs |

**Bulk run:** Select multiple connections via checkboxes → **Run N selected** button starts all audits simultaneously.

**Creating a new connection:** Click **New Connection** → Connection Form opens → choose **DigitalOcean** type.

#### DigitalOcean Connection Settings

| Field | Required | Description |
|-------|----------|-------------|
| **Name** | ✅ | Human-readable label (e.g. "Client X Production") |
| **DO API Token** | ✅ | DigitalOcean personal access token with read scope |
| **Project ID** | ⬜ | UUID from DO Dashboard → Projects. Leave blank for account-wide scan |
| **Scope mode** | ✅ | See below |
| **Spaces buckets** | ⬜ | Comma-separated: `bucket:region:sensitivity` |

**Scope modes:**

| Mode | What gets scanned |
|------|------------------|
| `project` | Only resources explicitly assigned to the DO project |
| `hybrid` | Project resources + resources inferred to belong to it (recommended) |
| `account` | Entire DO account — all droplets, databases, apps across all projects |

**Test connection** — validates the token against the DO API and lists accessible projects before saving.

---

### Security → Code & IaC

**Path:** `/code-iac`

Manage code repository connections and run code security scans.

Works identically to Cloud Audits but uses connections of type **Code**. Requires **Professional** plan or above.

#### Code Connection Settings

| Field | Required | Description |
|-------|----------|-------------|
| **Name** | ✅ | Label for this repo/project |
| **Source** | ✅ | `git` — clone from URL, or `local` — use a path on the server |
| **Repository URL** | git only | Full HTTPS/SSH URL (e.g. `https://github.com/org/repo`) |
| **Branch** | git only | Branch to scan (default: main) |
| **Access token** | git only | GitHub/GitLab token for private repos |
| **Local path** | local only | Absolute path on the server where the repo is already cloned |

**What gets scanned automatically:**

| Stack detected | Tools used |
|---------------|-----------|
| Any | gitleaks (secrets, credentials, API keys) |
| Node.js / TypeScript | semgrep + npm audit |
| Docker | semgrep (Dockerfile rules) |
| GitHub Actions | semgrep (CI/CD pipeline rules) |
| Terraform | trivy config + hclscan (custom DigitalOcean rules) |

---

### Security → Findings

**Path:** `/findings`

Aggregated view of all security findings across all audit jobs.

**Top stat cards:**
- Critical (red), High (orange), Medium (yellow), Low (blue) — counts with colored icons

**Filter bar:**
- By **severity** — Critical / High / Medium / Low
- By **connection** — filter to a specific system
- By **status** — open / acknowledged / resolved

**Findings table columns:**

| Column | Description |
|--------|-------------|
| Severity | Color-coded badge |
| Title | Short finding name (e.g. "Open port 22 to 0.0.0.0/0") |
| Resource | Affected droplet, database, file path, or domain |
| Connection | Which connection the finding came from |
| First seen | Date of the first audit that found it |
| Status | Open / Acknowledged / Resolved |

**Actions per row:**
- Click to expand full details: description, evidence, remediation steps
- Change status (acknowledge, resolve)

---

### Security → Remediation

**Path:** `/remediation`

Kanban board to track remediation of findings from audit to resolved.

**Columns:**
- **Backlog** — newly imported findings, not yet prioritized
- **In Progress** — assigned and being worked on
- **Review** — fix implemented, pending verification
- **Done** — fully resolved

**Task card shows:**
- Severity badge
- Title and description
- Due date (red if overdue)
- Assignee

**Actions:**
- Drag & drop between columns
- Set due date and assignee
- Add notes/comments
- Mark as done

**Dashboard integration:** Remediation progress % and overdue count appear on the Dashboard.

---

### Security → Monitoring

**Path:** `/monitoring`

Continuous security posture tracking across all connections.

**Security Score** per connection:
- Calculated from finding severity and count
- Score 0–100: < 60 red, 60–79 yellow, ≥ 80 green

**SLA tracking:**
- Set expected time-to-remediate per severity (e.g. Critical: 7 days)
- Red badge when a finding has exceeded its SLA deadline

**Overview widget** on Dashboard shows average score and SLA breach count.

---

### Compliance → Frameworks

**Path:** `/compliance`

Map your audit findings to compliance frameworks and track readiness.

**Supported frameworks:**

| Framework | Description |
|-----------|-------------|
| **SOC 2** | Trust Service Criteria (security, availability, confidentiality) |
| **ISO 27001** | Information Security Management System controls |
| **NIST CSF** | Cybersecurity Framework (Identify, Protect, Detect, Respond, Recover) |
| **CIS v8** | Center for Internet Security Controls v8 |

**Per framework view:**
- Total controls
- Met / Not met / Partially met
- Score % progress bar
- Evidence linked to controls

**How controls get met:**
- Automatically when a related audit finding is resolved
- Manually by linking evidence artifacts

---

### Compliance → Evidence

**Path:** `/evidence`

Document and store evidence artifacts for compliance audits.

**Evidence types:**
- Audit reports (auto-generated from CloudSecGuard jobs)
- Screenshots and documents (upload PDF/PNG)
- Policy documents
- Meeting minutes / approval records

**Fields:**
- Title, type, description
- Link to compliance control
- Uploaded file or URL
- Review date

---

### Compliance → Policies

**Path:** `/policies`

Write and manage internal security policies.

**Policy fields:**
- Title and category (Access Control, Data Protection, Incident Response, etc.)
- Content (rich text)
- Owner and approval status
- Review cycle (annual, semi-annual, etc.)
- Expiry date

**Statuses:** Draft → Review → Approved → Expired

**Dashboard widget** shows count of approved policies and flags expired ones in red.

---

### Compliance → Access Reviews

**Path:** `/access-reviews`

Periodic reviews of who has access to what systems.

**Review fields:**
- System / resource name
- Reviewer and owner
- Due date
- Access list items (user, role, justification, approved/revoked)

**Statuses:** Pending → In Progress → Completed

**Dashboard widget** shows overdue reviews and those due this month.

---

### Reports → Reports

**Path:** `/reports`

Complete audit history with download links for all finished reports.

**Table columns:**

| Column | Description |
|--------|-------------|
| Connection | Which system was audited |
| Type | Cloud / Code / SSL / DNS |
| Status | Running / Done / Failed |
| Started | Timestamp |
| Findings | C/H/M/L breakdown (done jobs only) |
| Actions | Download HTML, Download DOCX, Share link |

**Report types:**

| Format | Use case |
|--------|---------|
| **HTML** | View in browser, print to PDF, internal review |
| **DOCX** | Send to client, edit in Word/Google Docs |

**Share link** — generates a time-limited public URL that allows the client to view the report without logging in. See [Share Links](#share-links).

**Report sections (DigitalOcean audit):**
1. Executive Summary
2. Scope and methodology
3. Risk matrix
4. Findings (per resource, with severity and remediation steps)
5. Inventory (droplets, databases, apps, Spaces)
6. Appendix (raw evidence)

**Report sections (Code audit):**
1. Executive Summary
2. Secret exposure findings
3. Static analysis findings (per file/line)
4. Dependency vulnerabilities (npm audit)
5. Terraform/IaC misconfigurations
6. Appendix

---

### Reports → Audit Types

**Path:** `/audit-types`

Starting point for any new audit. Presents the four audit types as visual cards.

| Card | Description | Plan required |
|------|-------------|---------------|
| **DigitalOcean** | Cloud infrastructure audit | Free |
| **Code & IaC** | Repository and Terraform scan | Professional |
| **SSL/TLS** | Certificate and TLS configuration check | Free |
| **DNS** | DNS records security check | Free |

Clicking a card navigates to the respective connections page. If no connection exists yet, clicking **New Connection** opens the Connection Form.

---

### Account → Plans

**Path:** `/plans`

View plan tiers and enter a license key.

**Plan tiers:**

| Plan | Price | Connections | Audits/month | Users | Features |
|------|-------|-------------|--------------|-------|---------|
| **Free** | $0 | 5 | 20 | 1 | Basic audits, share links |
| **Starter** | $19/mo | 10 | 30 | 2 | + Scheduled audits, PDF reports |
| **Pro** | $49/mo | 30 | 100 | 5 | + Code audits, custom branding |
| **Business** | $99/mo | Unlimited | Unlimited | 15 | + API tokens, team management |
| **Enterprise** | Custom | Unlimited | Unlimited | Unlimited | + All features, priority support |

**Activating a license:**
1. Purchase at https://infrajump.com/pricing
2. Copy your license key (JWT format)
3. Open **Account → Plans** → scroll to **License Key** section
4. Paste the key → click **Activate**

The license is validated cryptographically (RSA signature). It becomes active immediately without a restart.

**Admin preview plan** (Settings → Team → Admin Preview):
- Admins and owners can preview any plan's features locally
- Used for testing gated UI without a real license
- Does not affect non-admin users

---

### Account → Settings

**Path:** `/settings`

Seven-tab vertical navigation covering all account and workspace configuration.

---

#### Settings → Profile

Personal information embedded in every report.

| Field | Description |
|-------|-------------|
| **Auditor name** | Displayed as "Prepared by" in reports |
| **Organization** | Company name in report header |
| **Email** | Auditor contact in report footer |
| **Phone** | Auditor phone in report footer |
| **Website** | Auditor website in report footer |
| **Address** | Auditor address for formal reports |
| **Email notifications** | Toggle to receive email when an audit job completes |

Click **Save changes** to apply.

---

#### Settings → Security

Account security controls.

**Change password:**
- Enter current password, new password (min 8 chars), confirm
- Click **Change password**

**Two-Factor Authentication (2FA / MFA):**
1. Click **Set up 2FA**
2. Scan the QR code with Google Authenticator / Authy / 1Password
3. Enter the 6-digit code to confirm
4. 2FA is now required at every login

To disable: Click **Disable 2FA** → enter current code to confirm.

**API Tokens** (Business/Enterprise plan):
- Generate tokens for programmatic access to the API
- Each token has a name and is shown once at creation
- Click **Revoke** to invalidate a token

---

#### Settings → Branding

Upload custom visual assets embedded in every generated report.

| Asset | Size/format | Used in |
|-------|-------------|---------|
| **Logo** | PNG, transparent background | Report header, cover page |
| **Watermark** | PNG, semi-transparent | Background of every report page |
| **Footer background** | PNG, 1920×200px or similar | Report footer band |

If no custom asset is uploaded, the default CloudSecGuard assets are used.

Click the upload area or drag a PNG file onto it. Preview shows immediately.

---

#### Settings → Team

Manage team members. Requires **Business** plan or above.

**Invite a member:**
1. Enter email address
2. Select role: **Viewer** (read-only) or **Auditor** (can run audits) or **Admin** (full access)
3. Click **Send invite**
4. The user receives an email with a one-time link to join

**Roles:**

| Role | Permissions |
|------|------------|
| **Viewer** | Read reports and findings only |
| **Auditor** | Run audits, manage connections, view all reports |
| **Admin** | Everything + manage team members and settings |
| **Owner** | Everything + billing and license |

**License section:**
- Shows current plan, limits, expiry
- Input field to enter / update a license key
- **Admin preview plan** dropdown — lets admins test plan features locally

---

#### Settings → External

Auditor portal configuration. Create access portals for external clients.

**Auditor invites:**
- Generate a portal invite link for a specific connection
- The client opens the link and sees a read-only view of their latest audit report
- Set permissions: view findings only, or full report

The portal URL is `/auditor/:token` — no login required.

---

#### Settings → Modules

Enable or disable platform modules. Allows trimming the sidebar to only show features your team uses.

**Available modules:**

| Module | Sidebar item affected |
|--------|----------------------|
| Cloud Audits | Security → Cloud Audits |
| Code & IaC | Security → Code & IaC |
| Findings | Security → Findings |
| Remediation | Security → Remediation |
| Monitoring | Security → Monitoring |
| Compliance | Compliance → Frameworks |
| Evidence | Compliance → Evidence |
| Policies | Compliance → Policies |
| Access Reviews | Compliance → Access Reviews |
| Reports | Reports → Reports |
| Audit Types | Reports → Audit Types |

Toggle the switch next to each module. Changes take effect immediately (no restart needed).

---

#### Settings → Activity

Audit log of recent workspace events.

Shows a timestamped list of:
- Logins and logouts
- Connection created/edited/deleted
- Audit jobs started and completed
- Settings changed
- Team member invited/removed
- License key activated

Useful for security incident investigation and compliance audit trails.

---

## Connection Types

Summary of all four connection types and what they produce:

| Type | Input | What's checked | Output |
|------|-------|---------------|--------|
| **DigitalOcean** | DO API token + project | Firewall rules, open ports, database exposure, Spaces bucket ACLs, App Platform secrets, TLS config | HTML + DOCX infrastructure report |
| **Code** | Git URL or local path | Hardcoded secrets, insecure code patterns, Dockerfile best practices, Terraform misconfigs, outdated npm packages | HTML + DOCX code security report |
| **SSL/TLS** | Domain(s) | Cert expiry, cert chain, protocol versions (TLS 1.0/1.1 = fail), weak ciphers, HSTS header, OCSP stapling | Findings in the platform |
| **DNS** | Domain(s) | SPF record presence/policy, DKIM, DMARC (p=reject/quarantine), DNSSEC, CAA record, zone transfer | Findings in the platform |

---

## Audit Job Lifecycle

```
New Connection
      │
      ▼
  Run Audit  ──────────────────────────────────┐
      │                                         │
      ▼                                         │
  queued → running → done                    failed
                │                               │
                ▼                               ▼
         Findings saved               Error logged (view in job detail)
                │
                ▼
        Reports generated
       (HTML + DOCX files)
                │
          ┌─────┴─────┐
          ▼           ▼
       Download    Share link
      (Reports)   (public URL)
```

**Job Detail page** (`/jobs/:id`):
- Real-time progress bar via WebSocket stream
- Live log output (scanner steps)
- Findings table (once done)
- Download buttons for HTML and DOCX
- Share button to generate a public link

---

## Share Links

A share link lets you send an audit report to a client without giving them a platform account.

**To create a share link:**
1. Go to **Reports** or open a finished job
2. Click the **Share** button
3. Copy the generated URL (format: `/share/:token`)

**The share view:**
- Read-only — client can view findings and the full report
- No login required
- Token is unique per job
- Can be revoked by deleting the connection (tokens expire with the job)

**Auditor Portal** (`/auditor/:token`) — a more formal white-labeled version sent via **Settings → External → Auditor Invites**.

---

## Auditor Portal

A white-labeled, read-only view for external clients.

**How it works:**
1. Admin generates an invite in **Settings → External**
2. Link is sent to the client (e.g. `https://yourdomain.com/auditor/abc123`)
3. Client opens the link — sees findings, severity breakdown, and report download
4. No account creation needed
5. Permissions are scoped to specific connections

---

## Plans & License

The platform enforces plan limits at the API level:

| Limit | Free | Starter | Pro | Business | Enterprise |
|-------|------|---------|-----|----------|------------|
| Connections | 5 | 10 | 30 | ∞ | ∞ |
| Audits/month | 20 | 30 | 100 | ∞ | ∞ |
| Users | 1 | 2 | 5 | 15 | ∞ |
| Scheduled audits | ❌ | ✅ | ✅ | ✅ | ✅ |
| Code audits | ❌ | ❌ | ✅ | ✅ | ✅ |
| Share links | ✅ | ✅ | ✅ | ✅ | ✅ |
| Custom branding | ❌ | ❌ | ✅ | ✅ | ✅ |
| API tokens | ❌ | ❌ | ❌ | ✅ | ✅ |
| Team management | ❌ | ❌ | ❌ | ✅ | ✅ |

When a feature is not available on your plan, the UI shows a lock icon and a toast notification guides you to upgrade.

---

## CLI Audit Engine

For running audits directly from the server without the web UI.

### Prerequisites

Install these tools on the server:

```bash
# gitleaks — secret scanning
brew install gitleaks   # or: go install github.com/gitleaks/gitleaks/v8@latest

# semgrep — static analysis
pip install semgrep

# trivy — IaC / container scanning
brew install trivy   # or: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh

# npm — for package audits
node --version && npm --version
```

Verify all are installed:
```bash
which gitleaks semgrep trivy npm
```

### DigitalOcean Infrastructure Scan

```bash
export DO_TOKEN="dop_v1_..."

cd CloudSecGuard && go run ./cmd/infra-audit all-do \
  --client   "Client Company Name" \
  --project  "Client Company Infrastructure Audit" \
  --do-project-id "UUID-from-DO-Dashboard" \
  --scope-mode hybrid \
  --spaces-buckets "bucket-name:nyc3:sensitive,bucket2:sfo3:public" \
  --prepared-by  "Your Name / Security Team" \
  --auditor-org  "Your Company" \
  --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone "+1 555 1234567" \
  --auditor-address $'123 Main St\nNew York, NY 10001' \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo       assets/logo.png \
  --watermark  assets/watermark.png \
  --footer-bg  assets/footer-bg.png \
  --out out/client-name
```

**Output:**
```
out/client-name/
  evidence/do_inventory.json        # Raw collected data
  report/
    client_project.html             # HTML report
    client_project.docx             # DOCX report
    client_project_findings.json    # Structured findings
```

**Scope mode options:**

| `--scope-mode` | Description |
|---------------|-------------|
| `project` | Only resources assigned to the DO project |
| `hybrid` | Project resources + inferred related resources (recommended) |
| `account` | All resources in the entire DO account |

**Spaces bucket format:**  
`bucket-name:region:sensitivity`  
Sensitivity values: `public` / `sensitive` / `internal`  
Multiple buckets: comma-separated.

---

### Code & IaC Scan

```bash
# 1. Clone the client repo (if not already local)
git clone https://github.com/client/repo.git ~/work/client-repo

# 2. Run the scanner
cd CloudSecGuard && go run ./cmd/code-audit \
  --repo      /home/you/work/client-repo \
  --client    "Client Company Name" \
  --project   "Client Company Code Security Audit" \
  --prepared-by  "Your Name / Security Team" \
  --auditor-org  "Your Company" \
  --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" \
  --auditor-phone "+1 555 1234567" \
  --auditor-address $'123 Main St\nNew York, NY 10001' \
  --classification "Confidential" \
  --period "May 2026 point-in-time assessment" \
  --logo       assets/logo.png \
  --watermark  assets/watermark.png \
  --footer-bg  assets/footer-bg.png \
  --out out/client-name-code
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

Run both scans for one client and get two ready-to-deliver documents:

```bash
export DO_TOKEN="dop_v1_..."
CLIENT="Acme Corp"
PROJECT_ID="be6bf36b-e07e-48ed-90e1-2604b1455e52"
REPO="/home/you/work/acme/backend"
OUT="out/acme-may-2026"

cd CloudSecGuard

# 1. Infrastructure
go run ./cmd/infra-audit all-do \
  --client "$CLIENT" --project "$CLIENT Infrastructure Audit" \
  --do-project-id "$PROJECT_ID" --scope-mode hybrid \
  --prepared-by "Your Name / Security Team" \
  --auditor-org "Your Company" --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" --auditor-phone "+1 555 1234567" \
  --auditor-address $'123 Main St\nNew York, NY 10001' \
  --classification "Confidential" --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png --watermark assets/watermark.png --footer-bg assets/footer-bg.png \
  --out "$OUT/infra"

# 2. Code
go run ./cmd/code-audit \
  --repo "$REPO" \
  --client "$CLIENT" --project "$CLIENT Code Security Audit" \
  --prepared-by "Your Name / Security Team" \
  --auditor-org "Your Company" --auditor-email "you@yourcompany.com" \
  --auditor-website "yourcompany.com" --auditor-phone "+1 555 1234567" \
  --auditor-address $'123 Main St\nNew York, NY 10001' \
  --classification "Confidential" --period "May 2026 point-in-time assessment" \
  --logo assets/logo.png --watermark assets/watermark.png --footer-bg assets/footer-bg.png \
  --out "$OUT/code"

echo "Reports:"
ls "$OUT/infra/report/"
ls "$OUT/code/report/"
```

**Deliverable files:**

| File | Contents |
|------|---------|
| `*_infrastructure_audit.docx` | DO infrastructure: firewall, databases, apps, Spaces |
| `*_code_security_audit.docx` | Code: secrets, Dockerfile, GitHub Actions, Terraform |

HTML versions are for internal review and browser-to-PDF printing.

---

## Environment Variables

**Web platform (`web/.env`):**

| Variable | Required | Description |
|----------|----------|-------------|
| `GOOGLE_CLIENT_ID` | ⬜ | Google OAuth app client ID |
| `GOOGLE_CLIENT_SECRET` | ⬜ | Google OAuth app client secret |
| `GOOGLE_REDIRECT_URL` | ⬜ | Must match Google Console redirect URI |

**Backend (set via env or docker-compose.yml):**

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `ASSETS_DIR` | `./assets` | Path to logo/watermark/footer-bg defaults |
| `DATA_DIR` | `./data` | Path for file storage (uploaded assets) |
| `JWT_SECRET` | auto-generated | Secret for signing access tokens |
| `PORT` | `8080` | Backend listen port |

---

## Required Tools (CLI mode)

Only needed when running CLI audit commands directly (not needed for the web platform Docker deployment):

| Tool | Purpose | Install |
|------|---------|---------|
| `gitleaks` | Scan for hardcoded secrets | `go install github.com/gitleaks/gitleaks/v8@latest` |
| `semgrep` | Static code analysis | `pip install semgrep` |
| `trivy` | Terraform / IaC scanning | https://aquasecurity.github.io/trivy/latest/getting-started/installation |
| `npm` | Node.js dependency audit | Bundled with Node.js |

---

## Support

- Issues: https://github.com/Quixgit/CloudSecGuard/issues
- Pricing: https://infrajump.com/pricing
- Docs: https://infrajump.com/docs
