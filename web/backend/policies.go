package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxPolicyUpload = 20 << 20 // 20 MB

// ── Types ─────────────────────────────────────────────────────────────────────

type Policy struct {
	ID                 string                 `json:"id"`
	UserID             string                 `json:"-"`
	TenantID           string                 `json:"tenant_id,omitempty"`
	Name               string                 `json:"name"`
	Category           string                 `json:"category"`
	TemplateSlug       string                 `json:"template_slug"`
	ContentHTML        string                 `json:"content_html,omitempty"`
	FilePath           string                 `json:"-"`
	FileName           string                 `json:"file_name"`
	Status             string                 `json:"status"`
	Version            int                    `json:"version"`
	ApprovedByUserID   *string                `json:"approved_by_user_id,omitempty"`
	ApprovedByEmail    string                 `json:"approved_by_email,omitempty"`
	ApprovedAt         *time.Time             `json:"approved_at,omitempty"`
	ReviewDate         *string                `json:"review_date,omitempty"`
	LastReviewedAt     *time.Time             `json:"last_reviewed_at,omitempty"`
	Controls           []PolicyControlMapping `json:"controls"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

type PolicyControlMapping struct {
	PolicyID      string `json:"policy_id"`
	FrameworkSlug string `json:"framework_slug"`
	ControlCode   string `json:"control_code"`
}

type PolicyTemplate struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// ── Templates ─────────────────────────────────────────────────────────────────

var policyTemplates = []PolicyTemplate{
	{Slug: "information-security", Name: "Information Security Policy", Category: "Information Security", Description: "Establishes the security requirements and responsibilities for protecting information assets."},
	{Slug: "access-control", Name: "Access Control Policy", Category: "Access Control", Description: "Defines rules for granting, reviewing, and revoking access to systems and data."},
	{Slug: "incident-response", Name: "Incident Response Plan", Category: "Incident Response", Description: "Procedures for detecting, reporting, and responding to security incidents."},
	{Slug: "change-management", Name: "Change Management Policy", Category: "Change Management", Description: "Controls the process for requesting, approving, and deploying changes to systems."},
	{Slug: "backup-recovery", Name: "Backup and Recovery Policy", Category: "Backup & Recovery", Description: "Defines backup schedules, retention periods, and recovery procedures."},
	{Slug: "data-retention", Name: "Data Retention Policy", Category: "Data Retention", Description: "Specifies how long data must be retained and procedures for secure disposal."},
	{Slug: "acceptable-use", Name: "Acceptable Use Policy", Category: "Acceptable Use", Description: "Outlines acceptable and prohibited use of company IT resources and data."},
	{Slug: "vendor-management", Name: "Third Party Vendor Policy", Category: "Vendor Management", Description: "Requirements for assessing, onboarding, and monitoring third-party vendors."},
	{Slug: "password", Name: "Password Policy", Category: "Password", Description: "Requirements for password complexity, rotation, and multi-factor authentication."},
	{Slug: "encryption", Name: "Encryption Policy", Category: "Encryption", Description: "Standards for encrypting data at rest and in transit across all systems."},
}

func templateBySlug(slug string) (PolicyTemplate, bool) {
	for _, t := range policyTemplates {
		if t.Slug == slug {
			return t, true
		}
	}
	return PolicyTemplate{}, false
}

// generatePolicyHTML fills placeholders into a template and returns HTML
func generatePolicyHTML(slug, companyName, effectiveDate, reviewDate, ownerName, ownerTitle string) string {
	body := policyBodies[slug]
	if body == "" {
		body = "<p>Policy content not available for this template.</p>"
	}
	r := strings.NewReplacer(
		"{{company_name}}", companyName,
		"{{effective_date}}", effectiveDate,
		"{{review_date}}", reviewDate,
		"{{owner_name}}", ownerName,
		"{{owner_title}}", ownerTitle,
	)
	tmpl, ok := templateBySlug(slug)
	title := slug
	if ok {
		title = tmpl.Name
	}
	return `<!DOCTYPE html>
<html><head><meta charset="UTF-8">
<style>
body{font-family:Arial,sans-serif;font-size:14px;line-height:1.6;color:#1a1a2e;max-width:900px;margin:0 auto;padding:40px}
h1{font-size:22px;border-bottom:2px solid #9edfde;padding-bottom:8px;color:#0d1f2d}
h2{font-size:16px;margin-top:28px;color:#0d1f2d}
h3{font-size:14px;margin-top:20px}
table{width:100%;border-collapse:collapse;margin:12px 0}
td,th{border:1px solid #ccc;padding:8px 12px;text-align:left;font-size:13px}
th{background:#f4f4f4;font-weight:600}
.meta-box{background:#f8f9fa;border:1px solid #e0e0e0;border-radius:6px;padding:14px 18px;margin:20px 0}
.meta-box table{margin:0}
.meta-box td{border:none;padding:4px 8px}
.meta-box td:first-child{color:#555;width:160px;font-weight:600}
ul,ol{padding-left:20px}
li{margin:4px 0}
p{margin:8px 0}
</style>
</head><body>
<h1>` + title + `</h1>
<div class="meta-box">
<table>
<tr><td>Organization</td><td>` + companyName + `</td></tr>
<tr><td>Effective Date</td><td>` + effectiveDate + `</td></tr>
<tr><td>Review Date</td><td>` + reviewDate + `</td></tr>
<tr><td>Policy Owner</td><td>` + ownerName + `, ` + ownerTitle + `</td></tr>
<tr><td>Version</td><td>1.0</td></tr>
<tr><td>Status</td><td>Draft</td></tr>
</table>
</div>
` + r.Replace(body) + `
</body></html>`
}

// ── Policy template bodies ────────────────────────────────────────────────────

var policyBodies = map[string]string{
	"information-security": `
<h2>1. Purpose and Scope</h2>
<p>This Information Security Policy establishes the security requirements and responsibilities of <strong>{{company_name}}</strong> for protecting its information assets against unauthorized access, disclosure, modification, or destruction. This policy applies to all employees, contractors, and third parties with access to {{company_name}} systems and data.</p>

<h2>2. Information Security Objectives</h2>
<ul>
<li>Protect the confidentiality, integrity, and availability of all information assets.</li>
<li>Comply with applicable laws, regulations, and contractual obligations.</li>
<li>Manage security risks to an acceptable level.</li>
<li>Enable the business to operate securely and with resilience.</li>
</ul>

<h2>3. Roles and Responsibilities</h2>
<table>
<tr><th>Role</th><th>Responsibility</th></tr>
<tr><td>Policy Owner ({{owner_name}}, {{owner_title}})</td><td>Maintain and enforce this policy; conduct annual reviews.</td></tr>
<tr><td>Management</td><td>Provide resources for security program; set tone and culture.</td></tr>
<tr><td>IT/Security Team</td><td>Implement technical controls; monitor for threats; respond to incidents.</td></tr>
<tr><td>All Employees</td><td>Comply with this policy; report suspected incidents; complete security training.</td></tr>
</table>

<h2>4. Information Classification</h2>
<p>All information assets must be classified as one of the following:</p>
<table>
<tr><th>Level</th><th>Description</th><th>Examples</th></tr>
<tr><td>Confidential</td><td>Highly sensitive; limited distribution</td><td>Customer PII, credentials, financial data</td></tr>
<tr><td>Internal</td><td>For internal use only; not for public release</td><td>Internal documents, employee data</td></tr>
<tr><td>Public</td><td>Approved for public distribution</td><td>Marketing materials, public documentation</td></tr>
</table>

<h2>5. Access Control</h2>
<p>Access to information systems shall be granted on a need-to-know and least-privilege basis. All access requests must be formally approved. Access rights shall be reviewed at least every 90 days and revoked promptly upon termination.</p>

<h2>6. Asset Management</h2>
<p>All information assets shall be inventoried and assigned an owner. Owners are responsible for ensuring appropriate classification and protection of their assets.</p>

<h2>7. Cryptography</h2>
<p>Confidential data must be encrypted at rest and in transit using industry-standard algorithms (AES-256 for at-rest, TLS 1.2+ for in-transit). Encryption keys must be managed through an approved key management process.</p>

<h2>8. Incident Management</h2>
<p>All suspected security incidents must be reported immediately to the security team. Incidents shall be managed in accordance with the Incident Response Plan. Post-incident reviews shall be conducted to prevent recurrence.</p>

<h2>9. Business Continuity</h2>
<p>Critical systems must have documented recovery procedures. Recovery Time Objectives (RTO) and Recovery Point Objectives (RPO) shall be defined and tested at least annually.</p>

<h2>10. Compliance and Audit</h2>
<p>Compliance with this policy shall be audited at least annually. Non-compliance may result in disciplinary action. Exceptions require written approval from the policy owner.</p>

<h2>11. Policy Review</h2>
<p>This policy shall be reviewed annually or following significant changes to the business environment. The next scheduled review is <strong>{{review_date}}</strong>.</p>
`,

	"access-control": `
<h2>1. Purpose</h2>
<p>This Access Control Policy defines how <strong>{{company_name}}</strong> manages access to its information systems, applications, and data. The goal is to ensure that only authorized individuals have access to resources, and that access is appropriate for their role.</p>

<h2>2. Scope</h2>
<p>This policy applies to all systems, applications, databases, and network resources owned or operated by {{company_name}}, and to all users (employees, contractors, third parties) who access these resources.</p>

<h2>3. Access Control Principles</h2>
<ul>
<li><strong>Least Privilege:</strong> Users are granted the minimum access required to perform their job functions.</li>
<li><strong>Need-to-Know:</strong> Access to sensitive information is restricted to those with a business need.</li>
<li><strong>Separation of Duties:</strong> Critical tasks requiring multiple approvals are separated across roles.</li>
<li><strong>Default Deny:</strong> Access is denied by default unless explicitly granted.</li>
</ul>

<h2>4. User Account Management</h2>
<table>
<tr><th>Activity</th><th>Requirement</th></tr>
<tr><td>New Account Creation</td><td>Requires manager approval and HR confirmation</td></tr>
<tr><td>Access Review</td><td>Quarterly review of all user access rights</td></tr>
<tr><td>Account Termination</td><td>Revoked within 24 hours of employment end</td></tr>
<tr><td>Privileged Access</td><td>Requires CISO/Security team approval; reviewed monthly</td></tr>
</table>

<h2>5. Authentication Requirements</h2>
<ul>
<li>All users must authenticate with a unique username and password.</li>
<li>Multi-factor authentication (MFA) is mandatory for all remote access, privileged accounts, and access to Confidential data.</li>
<li>Shared accounts are prohibited except for service accounts with documented justification.</li>
<li>Sessions must time out after 15 minutes of inactivity.</li>
</ul>

<h2>6. Remote Access</h2>
<p>Remote access to internal systems is permitted only via approved VPN or Zero Trust Network Access (ZTNA) solutions. All remote sessions must use MFA. Remote access must be logged and monitored.</p>

<h2>7. Privileged Access Management</h2>
<p>Administrative and privileged accounts must be separate from standard user accounts. Privileged access activities must be logged in a tamper-evident audit log. Use of privileged access must be minimized and time-limited where possible.</p>

<h2>8. Physical Access</h2>
<p>Physical access to data centers and server rooms is restricted to authorized personnel. Physical access logs must be maintained and reviewed monthly.</p>

<h2>9. Access Review and Recertification</h2>
<p>All access rights shall be reviewed quarterly by respective system owners and managers. Access that is no longer required must be revoked immediately upon identification.</p>

<h2>10. Policy Owner and Review</h2>
<p>This policy is owned by <strong>{{owner_name}}</strong>, {{owner_title}}. It is effective as of {{effective_date}} and is due for review on <strong>{{review_date}}</strong>.</p>
`,

	"incident-response": `
<h2>1. Purpose</h2>
<p>This Incident Response Plan provides <strong>{{company_name}}</strong> with a structured approach for detecting, responding to, and recovering from security incidents. Effective incident response minimizes damage and reduces recovery time and costs.</p>

<h2>2. Scope</h2>
<p>This plan applies to all security incidents affecting {{company_name}} systems, data, or operations, regardless of origin (internal or external).</p>

<h2>3. Incident Response Team (IRT)</h2>
<table>
<tr><th>Role</th><th>Responsibility</th></tr>
<tr><td>Incident Commander</td><td>Leads response; escalates to executive leadership; external communication</td></tr>
<tr><td>Security Analyst</td><td>Technical investigation; containment and eradication</td></tr>
<tr><td>IT Operations</td><td>System isolation; backup restoration; infrastructure support</td></tr>
<tr><td>Legal/Compliance</td><td>Regulatory notification; evidence preservation; legal guidance</td></tr>
<tr><td>Communications</td><td>Internal and external stakeholder communication</td></tr>
</table>

<h2>4. Incident Classification</h2>
<table>
<tr><th>Severity</th><th>Definition</th><th>Response SLA</th></tr>
<tr><td>Critical (P1)</td><td>Active breach, ransomware, widespread system outage</td><td>Immediate (within 1 hour)</td></tr>
<tr><td>High (P2)</td><td>Confirmed attack, data exposure risk, single system compromised</td><td>Within 4 hours</td></tr>
<tr><td>Medium (P3)</td><td>Suspicious activity, policy violation, potential vulnerability</td><td>Within 24 hours</td></tr>
<tr><td>Low (P4)</td><td>Informational alerts, minor policy deviations</td><td>Within 72 hours</td></tr>
</table>

<h2>5. Response Phases</h2>
<h3>5.1 Preparation</h3>
<p>Maintain updated contact lists, response tools, and playbooks. Conduct tabletop exercises at least annually.</p>

<h3>5.2 Identification</h3>
<p>Detect and confirm incidents via SIEM alerts, user reports, or third-party notifications. Document the initial finding with timestamp, source, and scope.</p>

<h3>5.3 Containment</h3>
<p>Isolate affected systems to prevent spread. Preserve evidence (forensic images, logs). Implement short-term and long-term containment strategies.</p>

<h3>5.4 Eradication</h3>
<p>Remove malware, unauthorized accounts, or vulnerabilities. Validate that the root cause has been eliminated.</p>

<h3>5.5 Recovery</h3>
<p>Restore systems from clean backups. Monitor for signs of re-infection. Gradually return systems to production.</p>

<h3>5.6 Post-Incident Review</h3>
<p>Conduct a lessons-learned review within 2 weeks. Update playbooks, controls, and training as needed. Document the final incident report.</p>

<h2>6. Reporting Requirements</h2>
<ul>
<li>All employees must report suspected incidents immediately to security@{{company_name | lowercase}}.com or the helpdesk.</li>
<li>Data breaches involving personal data must be reported to the DPO within 24 hours of detection.</li>
<li>Regulatory notifications (e.g., GDPR 72-hour rule) must be coordinated with Legal.</li>
</ul>

<h2>7. Plan Owner and Review</h2>
<p>This plan is owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"change-management": `
<h2>1. Purpose</h2>
<p>This Change Management Policy establishes the process by which <strong>{{company_name}}</strong> manages changes to its IT infrastructure, systems, and applications. The goal is to minimize disruption, reduce risk, and maintain system integrity.</p>

<h2>2. Scope</h2>
<p>This policy applies to all changes to production systems, including infrastructure changes, software deployments, configuration changes, and third-party integrations.</p>

<h2>3. Change Categories</h2>
<table>
<tr><th>Type</th><th>Description</th><th>Approval Required</th></tr>
<tr><td>Standard</td><td>Pre-approved, low-risk, repeatable changes</td><td>Pre-approved by CAB</td></tr>
<tr><td>Normal</td><td>Planned changes following full process</td><td>Change Advisory Board (CAB)</td></tr>
<tr><td>Emergency</td><td>Urgent changes to restore service or address critical risk</td><td>Emergency CAB or CISO</td></tr>
</table>

<h2>4. Change Process</h2>
<ol>
<li><strong>Request:</strong> Submit a Change Request (CR) describing the change, rationale, risk assessment, and rollback plan.</li>
<li><strong>Impact Assessment:</strong> Evaluate business, technical, and security impact.</li>
<li><strong>Approval:</strong> Obtain appropriate approvals based on change category.</li>
<li><strong>Implementation:</strong> Execute change during approved maintenance window.</li>
<li><strong>Testing:</strong> Validate change in staging environment before production deployment.</li>
<li><strong>Review:</strong> Post-implementation review within 48 hours to confirm success and document lessons learned.</li>
</ol>

<h2>5. Change Advisory Board (CAB)</h2>
<p>The CAB reviews and approves Normal changes. It meets weekly and includes representatives from IT, Security, Operations, and Business stakeholders. Emergency changes must be reviewed by the CAB post-implementation.</p>

<h2>6. Rollback Planning</h2>
<p>All changes must include a documented rollback plan. The rollback plan must be tested in staging before implementation. A rollback must be executed if the change causes unexpected issues or SLA breaches.</p>

<h2>7. Emergency Changes</h2>
<p>Emergency changes may be implemented with verbal approval from the CISO or IT Director. Full documentation must be completed within 24 hours. All emergency changes must be reviewed at the next CAB meeting.</p>

<h2>8. Audit and Compliance</h2>
<p>All change records must be retained for at least 3 years. Changes must be traceable to an approved CR. Unauthorized changes are a policy violation subject to disciplinary action.</p>

<h2>9. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"backup-recovery": `
<h2>1. Purpose</h2>
<p>This Backup and Recovery Policy defines the requirements for backing up critical data and systems at <strong>{{company_name}}</strong> and ensuring the ability to recover from data loss or system failures.</p>

<h2>2. Scope</h2>
<p>This policy applies to all production systems, databases, and data stores containing business-critical or regulated data.</p>

<h2>3. Backup Requirements</h2>
<table>
<tr><th>Data Type</th><th>Backup Frequency</th><th>Retention Period</th><th>Location</th></tr>
<tr><td>Databases (production)</td><td>Daily full + continuous WAL</td><td>90 days</td><td>Encrypted off-site / cloud</td></tr>
<tr><td>Application data</td><td>Daily incremental; Weekly full</td><td>30 days incremental; 1 year full</td><td>Encrypted off-site</td></tr>
<tr><td>Configuration files</td><td>On change + daily</td><td>1 year</td><td>Version-controlled repository</td></tr>
<tr><td>Logs / audit trails</td><td>Real-time streaming</td><td>1 year minimum</td><td>SIEM / log management</td></tr>
</table>

<h2>4. Recovery Objectives</h2>
<table>
<tr><th>System Tier</th><th>RTO (Recovery Time)</th><th>RPO (Recovery Point)</th></tr>
<tr><td>Tier 1 — Critical</td><td>4 hours</td><td>1 hour</td></tr>
<tr><td>Tier 2 — Important</td><td>24 hours</td><td>4 hours</td></tr>
<tr><td>Tier 3 — Standard</td><td>72 hours</td><td>24 hours</td></tr>
</table>

<h2>5. Backup Storage and Security</h2>
<ul>
<li>All backups must be encrypted at rest using AES-256.</li>
<li>Backup media must be stored in a geographically separate location from production systems.</li>
<li>Access to backup systems must be restricted to authorized personnel only.</li>
<li>Backup integrity must be verified automatically after each backup job.</li>
</ul>

<h2>6. Backup Testing and Verification</h2>
<p>Recovery procedures must be tested at least quarterly for Tier 1 systems and semi-annually for Tier 2/3. Test results must be documented and reported to management. Failed recovery tests must be investigated and remediated immediately.</p>

<h2>7. Responsibilities</h2>
<ul>
<li><strong>IT Operations:</strong> Execute backup jobs; monitor success/failure; resolve backup failures within 4 hours.</li>
<li><strong>System Owners:</strong> Define criticality and recovery requirements for their systems.</li>
<li><strong>Security Team:</strong> Ensure backup encryption and access controls are in place.</li>
</ul>

<h2>8. Incident Response Integration</h2>
<p>In the event of data loss, the Incident Response Plan must be activated. Restores from backup must be documented and authorized before execution in production.</p>

<h2>9. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"data-retention": `
<h2>1. Purpose</h2>
<p>This Data Retention Policy specifies the minimum and maximum periods for which <strong>{{company_name}}</strong> retains different categories of data, and the procedures for secure disposal of data that is no longer required.</p>

<h2>2. Scope</h2>
<p>This policy applies to all data created, received, maintained, or transmitted by {{company_name}}, regardless of format (electronic, paper, or other media).</p>

<h2>3. Data Retention Schedule</h2>
<table>
<tr><th>Data Category</th><th>Retention Period</th><th>Notes</th></tr>
<tr><td>Customer contracts and agreements</td><td>7 years after expiry</td><td>Legal obligation</td></tr>
<tr><td>Financial records</td><td>7 years</td><td>Tax and audit requirements</td></tr>
<tr><td>Employee records</td><td>7 years after termination</td><td>Employment law</td></tr>
<tr><td>Security logs / audit trails</td><td>1 year minimum; 3 years preferred</td><td>Incident investigation</td></tr>
<tr><td>Customer personal data (PII)</td><td>Duration of relationship + 3 years</td><td>GDPR / privacy compliance</td></tr>
<tr><td>Email communications</td><td>3 years</td><td>Business records</td></tr>
<tr><td>System configuration backups</td><td>1 year</td><td>Recovery purposes</td></tr>
<tr><td>Marketing data</td><td>Until consent withdrawn + 1 year</td><td>GDPR consent</td></tr>
</table>

<h2>4. Legal Hold</h2>
<p>When litigation, regulatory investigation, or audit is anticipated, normal retention schedules are suspended. The Legal team must notify relevant data custodians of legal holds immediately. Data under legal hold must not be altered or destroyed.</p>

<h2>5. Data Disposal</h2>
<ul>
<li><strong>Electronic data:</strong> Must be securely wiped using NIST SP 800-88 methods or cryptographic erasure.</li>
<li><strong>Paper records:</strong> Must be shredded using a cross-cut shredder; third-party shredding services require a certificate of destruction.</li>
<li><strong>Storage media:</strong> Degaussing or physical destruction required before disposal or reuse.</li>
</ul>

<h2>6. Responsibilities</h2>
<ul>
<li>Data owners are responsible for ensuring data in their area is retained and disposed of per this policy.</li>
<li>IT is responsible for implementing technical controls supporting retention and disposal.</li>
<li>Legal/Compliance is responsible for managing legal holds and regulatory requirements.</li>
</ul>

<h2>7. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"acceptable-use": `
<h2>1. Purpose</h2>
<p>This Acceptable Use Policy (AUP) defines the acceptable and prohibited uses of <strong>{{company_name}}</strong> information technology resources, including hardware, software, networks, and data. All users are required to comply with this policy.</p>

<h2>2. Scope</h2>
<p>This policy applies to all employees, contractors, consultants, and any other persons using {{company_name}} IT resources, whether on-premises or remotely.</p>

<h2>3. Acceptable Use</h2>
<ul>
<li>Using company systems for authorized business purposes.</li>
<li>Accessing the internet for work-related research and communication.</li>
<li>Using company email for professional communication.</li>
<li>Installing software that has been approved by IT.</li>
<li>Reporting suspected security incidents and policy violations.</li>
</ul>

<h2>4. Prohibited Activities</h2>
<ul>
<li>Using company systems for unauthorized personal business or illegal activities.</li>
<li>Accessing, downloading, or distributing inappropriate, offensive, or illegal content.</li>
<li>Sharing credentials or allowing others to use your account.</li>
<li>Installing unauthorized software, including personal applications.</li>
<li>Circumventing or disabling security controls.</li>
<li>Accessing systems or data without proper authorization.</li>
<li>Mining cryptocurrency or using company resources for personal financial gain.</li>
<li>Sending confidential company data to personal email accounts or unauthorized storage.</li>
<li>Using social media to disclose confidential company information.</li>
</ul>

<h2>5. Internet and Email Use</h2>
<p>Internet and email access is provided for business purposes. Limited personal use is permitted provided it does not interfere with work duties or violate this policy. All internet and email activity on company systems may be monitored.</p>

<h2>6. Mobile Devices and Remote Work</h2>
<p>Company-issued mobile devices must have screen locks and encryption enabled. Personal devices used for work (BYOD) must comply with the Mobile Device Policy and be enrolled in Mobile Device Management (MDM). Public Wi-Fi must not be used without VPN.</p>

<h2>7. Monitoring</h2>
<p>{{company_name}} reserves the right to monitor, intercept, and review all activity on company IT systems. Users have no expectation of privacy when using company resources. Monitoring is conducted in accordance with applicable laws.</p>

<h2>8. Violations</h2>
<p>Violations of this policy may result in disciplinary action, up to and including termination of employment or contract, and potentially legal action.</p>

<h2>9. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"vendor-management": `
<h2>1. Purpose</h2>
<p>This Third Party Vendor Policy establishes the requirements for <strong>{{company_name}}</strong> to assess, onboard, monitor, and offboard third-party vendors and service providers who have access to company systems or data.</p>

<h2>2. Scope</h2>
<p>This policy applies to all third-party vendors, suppliers, contractors, and service providers who access, store, process, or transmit {{company_name}} data or systems.</p>

<h2>3. Vendor Risk Classification</h2>
<table>
<tr><th>Tier</th><th>Description</th><th>Assessment Frequency</th></tr>
<tr><td>Critical</td><td>Access to confidential data; critical infrastructure dependencies</td><td>Annual full assessment</td></tr>
<tr><td>High</td><td>Access to internal systems; limited data access</td><td>Annual questionnaire</td></tr>
<tr><td>Low</td><td>No system or data access; commodity services</td><td>Onboarding only</td></tr>
</table>

<h2>4. Vendor Onboarding Requirements</h2>
<ul>
<li>All vendors must complete a security questionnaire before contract execution.</li>
<li>Critical and High-tier vendors must provide evidence of security certifications (e.g., SOC 2, ISO 27001).</li>
<li>Data Processing Agreements (DPAs) are required for any vendor handling personal data.</li>
<li>Contracts must include security requirements, audit rights, and breach notification obligations.</li>
</ul>

<h2>5. Ongoing Monitoring</h2>
<ul>
<li>Critical vendors must be reassessed annually with a full security review.</li>
<li>Vendors must notify {{company_name}} of security incidents affecting company data within 24 hours.</li>
<li>Third-party access must be logged and reviewed quarterly.</li>
<li>Vendor access must follow least-privilege principles and be limited to what is necessary.</li>
</ul>

<h2>6. Vendor Offboarding</h2>
<p>Upon contract termination, all vendor access must be revoked within 24 hours. Vendors must confirm deletion of {{company_name}} data within 30 days per contractual terms. Return or destruction of physical assets must be documented.</p>

<h2>7. Sub-Processors</h2>
<p>Vendors must disclose any sub-processors with access to {{company_name}} data. Sub-processors must meet the same security standards as the primary vendor. {{company_name}} retains the right to object to sub-processor changes.</p>

<h2>8. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"password": `
<h2>1. Purpose</h2>
<p>This Password Policy establishes the requirements for creating, maintaining, and protecting passwords used to access <strong>{{company_name}}</strong> systems and data. Strong password practices are a fundamental control against unauthorized access.</p>

<h2>2. Scope</h2>
<p>This policy applies to all users (employees, contractors, service accounts) who access {{company_name}} systems requiring password authentication.</p>

<h2>3. Password Requirements</h2>
<table>
<tr><th>Requirement</th><th>Standard</th></tr>
<tr><td>Minimum length</td><td>12 characters (16 recommended)</td></tr>
<tr><td>Complexity</td><td>Must include uppercase, lowercase, number, and special character</td></tr>
<tr><td>Prohibited passwords</td><td>Dictionary words, usernames, company name, sequential patterns (123, abc)</td></tr>
<tr><td>Password reuse</td><td>Last 12 passwords may not be reused</td></tr>
<tr><td>Maximum age</td><td>90 days for standard accounts; 30 days for privileged accounts</td></tr>
<tr><td>Account lockout</td><td>Lock after 5 failed attempts; 30-minute auto-unlock or admin reset</td></tr>
</table>

<h2>4. Multi-Factor Authentication (MFA)</h2>
<p>MFA is mandatory for:</p>
<ul>
<li>All remote access (VPN, RDP, SSH).</li>
<li>All privileged and administrative accounts.</li>
<li>All access to cloud management consoles.</li>
<li>All SaaS applications holding Confidential data.</li>
</ul>
<p>Approved MFA methods: TOTP authenticator apps, hardware security keys (FIDO2). SMS-based OTP is permitted only as a fallback and is discouraged.</p>

<h2>5. Password Storage and Transmission</h2>
<ul>
<li>Passwords must never be stored in plaintext; only salted cryptographic hashes (bcrypt, Argon2) are acceptable.</li>
<li>Passwords must never be transmitted in plaintext; TLS 1.2 or higher is required.</li>
<li>Passwords must not be shared, written down, or embedded in code or scripts.</li>
<li>All shared credentials must be stored in an approved password manager.</li>
</ul>

<h2>6. Service Account Passwords</h2>
<p>Service account credentials must be at least 32 characters long, randomly generated, and stored in a secrets management system (e.g., HashiCorp Vault, AWS Secrets Manager). Service account passwords must be rotated at least annually.</p>

<h2>7. Password Manager</h2>
<p>{{company_name}} provides an approved enterprise password manager. All employees must use it for storing work-related credentials. Storing passwords in web browsers is acceptable only for low-risk accounts.</p>

<h2>8. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,

	"encryption": `
<h2>1. Purpose</h2>
<p>This Encryption Policy establishes the standards for protecting sensitive data at <strong>{{company_name}}</strong> through encryption, ensuring confidentiality and integrity of data at rest and in transit.</p>

<h2>2. Scope</h2>
<p>This policy applies to all systems, applications, and communications that handle Confidential or Internal data as classified under the Information Security Policy.</p>

<h2>3. Encryption Standards</h2>
<table>
<tr><th>Use Case</th><th>Required Standard</th><th>Notes</th></tr>
<tr><td>Data at rest (databases)</td><td>AES-256</td><td>Column-level or full-disk encryption</td></tr>
<tr><td>Data at rest (file storage)</td><td>AES-256</td><td>Server-side encryption mandatory</td></tr>
<tr><td>Data in transit (web)</td><td>TLS 1.2 minimum; TLS 1.3 preferred</td><td>HSTS must be enabled</td></tr>
<tr><td>Data in transit (email)</td><td>TLS; PGP/S-MIME for confidential content</td><td>—</td></tr>
<tr><td>Data in transit (APIs)</td><td>TLS 1.2+ with mutual TLS for internal APIs</td><td>Certificate pinning for mobile apps</td></tr>
<tr><td>Disk / endpoint encryption</td><td>BitLocker, FileVault, or equivalent</td><td>Mandatory on all laptops</td></tr>
<tr><td>Code signing</td><td>RSA-2048 or ECDSA P-256 minimum</td><td>All production deployments</td></tr>
</table>

<h2>4. Prohibited Algorithms</h2>
<p>The following algorithms are deprecated and must not be used:</p>
<ul>
<li>DES, 3DES, RC4, MD5 (for security purposes), SHA-1 (for digital signatures), SSL 2.0/3.0, TLS 1.0/1.1.</li>
</ul>

<h2>5. Key Management</h2>
<ul>
<li>Encryption keys must be managed using an approved Key Management System (KMS).</li>
<li>Key length: RSA minimum 2048-bit; ECDSA minimum 256-bit; AES minimum 256-bit.</li>
<li>Symmetric keys must be rotated at least annually or immediately upon suspected compromise.</li>
<li>TLS certificates must be renewed before expiry; certificate expiry monitoring is mandatory.</li>
<li>Key custodians must be designated and documented for all critical keys.</li>
<li>Private keys must never be stored in version control or shared insecurely.</li>
</ul>

<h2>6. Certificate Management</h2>
<p>All public-facing TLS certificates must be issued by a trusted Certificate Authority (CA). Self-signed certificates are permitted only in non-production environments. Certificate inventory must be maintained and reviewed quarterly.</p>

<h2>7. Mobile and Endpoint Devices</h2>
<p>All laptops and mobile devices containing company data must have full-disk encryption enabled. Encryption status must be verifiable via MDM. Devices failing encryption compliance checks must be blocked from accessing corporate resources.</p>

<h2>8. Exceptions</h2>
<p>Any exception to this policy requires written approval from the CISO and must be documented with compensating controls and a defined remediation timeline.</p>

<h2>9. Policy Owner and Review</h2>
<p>Owned by <strong>{{owner_name}}</strong>, {{owner_title}}. Effective: {{effective_date}}. Next review: <strong>{{review_date}}</strong>.</p>
`,
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (srv *server) listPolicies(ctx context.Context, tenantID string) ([]Policy, error) {
	rows, err := srv.db.Query(ctx, `
		SELECT p.id, p.user_id, p.tenant_id, p.name, p.category, p.template_slug,
		       p.content_html, p.file_path, p.file_name, p.status, p.version,
		       p.approved_by_user_id, COALESCE(u2.email,'') as approved_by_email, p.approved_at,
		       to_char(p.review_date,'YYYY-MM-DD'), p.last_reviewed_at, p.created_at, p.updated_at
		FROM policies p
		LEFT JOIN users u2 ON u2.id = p.approved_by_user_id
		WHERE p.tenant_id = $1
		ORDER BY p.created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Policy
	for rows.Next() {
		var p Policy
		err := rows.Scan(
			&p.ID, &p.UserID, &p.TenantID, &p.Name, &p.Category, &p.TemplateSlug,
			&p.ContentHTML, &p.FilePath, &p.FileName, &p.Status, &p.Version,
			&p.ApprovedByUserID, &p.ApprovedByEmail, &p.ApprovedAt,
			&p.ReviewDate, &p.LastReviewedAt, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach control mappings
	for i := range out {
		mappings, err := srv.getPolicyControls(ctx, out[i].ID)
		if err == nil {
			out[i].Controls = mappings
		}
		if out[i].Controls == nil {
			out[i].Controls = []PolicyControlMapping{}
		}
	}
	return out, nil
}

func (srv *server) getPolicy(ctx context.Context, id, tenantID string) (Policy, error) {
	var p Policy
	err := srv.db.QueryRow(ctx, `
		SELECT p.id, p.user_id, p.tenant_id, p.name, p.category, p.template_slug,
		       p.content_html, p.file_path, p.file_name, p.status, p.version,
		       p.approved_by_user_id, COALESCE(u2.email,'') as approved_by_email, p.approved_at,
		       to_char(p.review_date,'YYYY-MM-DD'), p.last_reviewed_at, p.created_at, p.updated_at
		FROM policies p
		LEFT JOIN users u2 ON u2.id = p.approved_by_user_id
		WHERE p.id = $1 AND p.tenant_id = $2`,
		id, tenantID,
	).Scan(
		&p.ID, &p.UserID, &p.TenantID, &p.Name, &p.Category, &p.TemplateSlug,
		&p.ContentHTML, &p.FilePath, &p.FileName, &p.Status, &p.Version,
		&p.ApprovedByUserID, &p.ApprovedByEmail, &p.ApprovedAt,
		&p.ReviewDate, &p.LastReviewedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return p, err
	}
	controls, err := srv.getPolicyControls(ctx, p.ID)
	if err == nil {
		p.Controls = controls
	}
	if p.Controls == nil {
		p.Controls = []PolicyControlMapping{}
	}
	return p, nil
}

func (srv *server) createPolicy(ctx context.Context, tenantID, userID string, req createPolicyRequest) (Policy, error) {
	var id string
	err := srv.db.QueryRow(ctx, `
		INSERT INTO policies(tenant_id, user_id, name, category, template_slug, content_html,
		                     file_path, file_name, status, version, review_date)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,'Draft',1,$9)
		RETURNING id`,
		tenantID, userID, req.Name, req.Category, req.TemplateSlug, req.ContentHTML,
		req.FilePath, req.FileName, req.ReviewDate,
	).Scan(&id)
	if err != nil {
		return Policy{}, err
	}
	if len(req.Controls) > 0 {
		_ = srv.setPolicyControls(ctx, id, req.Controls)
	}
	return srv.getPolicy(ctx, id, tenantID)
}

func (srv *server) updatePolicy(ctx context.Context, id, tenantID string, req updatePolicyRequest) (Policy, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE policies
		SET name=$3, category=$4, content_html=$5, file_name=$6,
		    status=$7, review_date=$8, updated_at=NOW(), version=version+1
		WHERE id=$1 AND tenant_id=$2`,
		id, tenantID, req.Name, req.Category, req.ContentHTML,
		req.FileName, req.Status, req.ReviewDate,
	)
	if err != nil {
		return Policy{}, err
	}
	if req.Controls != nil {
		_ = srv.setPolicyControls(ctx, id, req.Controls)
	}
	return srv.getPolicy(ctx, id, tenantID)
}

func (srv *server) approvePolicy(ctx context.Context, id, tenantID, approverUserID string) (Policy, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE policies
		SET status='Approved', approved_by_user_id=$3, approved_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2`,
		id, tenantID, approverUserID,
	)
	if err != nil {
		return Policy{}, err
	}
	return srv.getPolicy(ctx, id, tenantID)
}

func (srv *server) markPolicyReviewed(ctx context.Context, id, tenantID, newReviewDate string) (Policy, error) {
	_, err := srv.db.Exec(ctx, `
		UPDATE policies
		SET last_reviewed_at=NOW(), review_date=$3, updated_at=NOW()
		WHERE id=$1 AND tenant_id=$2`,
		id, tenantID, newReviewDate,
	)
	if err != nil {
		return Policy{}, err
	}
	return srv.getPolicy(ctx, id, tenantID)
}

func (srv *server) deletePolicy(ctx context.Context, id, tenantID string) error {
	// Delete uploaded file if exists
	var filePath string
	_ = srv.db.QueryRow(ctx, `SELECT file_path FROM policies WHERE id=$1 AND tenant_id=$2`, id, tenantID).Scan(&filePath)
	if filePath != "" {
		_ = os.Remove(filePath)
	}
	tag, err := srv.db.Exec(ctx, `DELETE FROM policies WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("not found")
	}
	return nil
}

func (srv *server) getPolicyControls(ctx context.Context, policyID string) ([]PolicyControlMapping, error) {
	rows, err := srv.db.Query(ctx,
		`SELECT policy_id, framework_slug, control_code FROM policy_control_mappings WHERE policy_id=$1`,
		policyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PolicyControlMapping
	for rows.Next() {
		var m PolicyControlMapping
		if err := rows.Scan(&m.PolicyID, &m.FrameworkSlug, &m.ControlCode); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (srv *server) setPolicyControls(ctx context.Context, policyID string, controls []PolicyControlMapping) error {
	tx, err := srv.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `DELETE FROM policy_control_mappings WHERE policy_id=$1`, policyID)
	if err != nil {
		return err
	}
	for _, c := range controls {
		_, err = tx.Exec(ctx,
			`INSERT INTO policy_control_mappings(policy_id, framework_slug, control_code)
			 VALUES($1,$2,$3) ON CONFLICT DO NOTHING`,
			policyID, c.FrameworkSlug, c.ControlCode)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ── Request / response types ──────────────────────────────────────────────────

type createPolicyRequest struct {
	Name         string                 `json:"name"`
	Category     string                 `json:"category"`
	TemplateSlug string                 `json:"template_slug"`
	ContentHTML  string                 `json:"content_html"`
	FilePath     string                 `json:"file_path"`
	FileName     string                 `json:"file_name"`
	ReviewDate   *string                `json:"review_date"`
	Controls     []PolicyControlMapping `json:"controls"`
	// template generation fields
	CompanyName   string `json:"company_name"`
	EffectiveDate string `json:"effective_date"`
	OwnerName     string `json:"owner_name"`
	OwnerTitle    string `json:"owner_title"`
}

type updatePolicyRequest struct {
	Name        string                 `json:"name"`
	Category    string                 `json:"category"`
	ContentHTML string                 `json:"content_html"`
	FileName    string                 `json:"file_name"`
	Status      string                 `json:"status"`
	ReviewDate  *string                `json:"review_date"`
	Controls    []PolicyControlMapping `json:"controls"`
}

type approvePolicyRequest struct{}

type reviewPolicyRequest struct {
	ReviewDate string `json:"review_date"`
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (srv *server) handleListPolicyTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, policyTemplates)
}

func (srv *server) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	policies, err := srv.listPolicies(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	if policies == nil {
		policies = []Policy{}
	}
	writeJSON(w, http.StatusOK, policies)
}

func (srv *server) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)

	var req createPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// If template slug provided and no content, generate it
	if req.TemplateSlug != "" && req.ContentHTML == "" {
		tmpl, ok := templateBySlug(req.TemplateSlug)
		if ok {
			if req.Name == "" {
				req.Name = tmpl.Name
			}
			if req.Category == "" {
				req.Category = tmpl.Category
			}
			reviewDate := req.ReviewDate
			rd := ""
			if reviewDate != nil {
				rd = *reviewDate
			}
			req.ContentHTML = generatePolicyHTML(
				req.TemplateSlug,
				req.CompanyName,
				req.EffectiveDate,
				rd,
				req.OwnerName,
				req.OwnerTitle,
			)
		}
	}

	policy, err := srv.createPolicy(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (srv *server) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	var req updatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	policy, err := srv.updatePolicy(r.Context(), id, tenantID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (srv *server) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	if err := srv.deletePolicy(r.Context(), id, tenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (srv *server) handleApprovePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)
	id := chi.URLParam(r, "id")

	policy, err := srv.approvePolicy(r.Context(), id, tenantID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "approve failed")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (srv *server) handleReviewPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	var req reviewPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	policy, err := srv.markPolicyReviewed(r.Context(), id, tenantID, req.ReviewDate)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "review failed")
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (srv *server) handleUploadPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	userID := r.Context().Value(ctxUserID).(string)

	if err := r.ParseMultipartForm(maxPolicyUpload); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, fh, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = fh.Filename
	}
	category := r.FormValue("category")
	reviewDate := r.FormValue("review_date")

	// Save to disk
	dataDir := envOr("DATA_DIR", "/app/data")
	policyDir := filepath.Join(dataDir, "tenants", tenantID, "policies")
	if err := os.MkdirAll(policyDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}
	safeName := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), filepath.Base(fh.Filename))
	destPath := filepath.Join(policyDir, safeName)
	out, err := os.Create(destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "write error")
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		writeError(w, http.StatusInternalServerError, "write error")
		return
	}

	// Read content type for HTML files
	contentHTML := ""
	if strings.HasSuffix(strings.ToLower(fh.Filename), ".html") || strings.HasSuffix(strings.ToLower(fh.Filename), ".htm") {
		if data, err := os.ReadFile(destPath); err == nil {
			contentHTML = string(data)
		}
	}

	var rd *string
	if reviewDate != "" {
		rd = &reviewDate
	}
	req := createPolicyRequest{
		Name:        name,
		Category:    category,
		FilePath:    destPath,
		FileName:    fh.Filename,
		ContentHTML: contentHTML,
		ReviewDate:  rd,
	}
	policy, err := srv.createPolicy(r.Context(), tenantID, userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (srv *server) handleDownloadPolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	id := chi.URLParam(r, "id")

	policy, err := srv.getPolicy(r.Context(), id, tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "policy not found")
		return
	}

	// If there's an uploaded file, serve it
	if policy.FilePath != "" {
		if _, err := os.Stat(policy.FilePath); err == nil {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, policy.FileName))
			http.ServeFile(w, r, policy.FilePath)
			return
		}
	}

	// Otherwise serve the HTML content
	filename := strings.ReplaceAll(strings.ToLower(policy.Name), " ", "_") + ".html"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write([]byte(policy.ContentHTML))
}

// ── Stats for dashboard ───────────────────────────────────────────────────────

type PolicyStats struct {
	Total      int `json:"total"`
	Approved   int `json:"approved"`
	Draft      int `json:"draft"`
	Expired    int `json:"expired"`
	ReviewDue  int `json:"review_due"`
}

func (srv *server) getPolicyStats(ctx context.Context, tenantID string) (PolicyStats, error) {
	var s PolicyStats
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE tenant_id=$1`, tenantID).Scan(&s.Total)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE tenant_id=$1 AND status='Approved'`, tenantID).Scan(&s.Approved)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE tenant_id=$1 AND status='Draft'`, tenantID).Scan(&s.Draft)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE tenant_id=$1 AND status='Expired'`, tenantID).Scan(&s.Expired)
	_ = srv.db.QueryRow(ctx, `SELECT COUNT(*) FROM policies WHERE tenant_id=$1 AND review_date IS NOT NULL AND review_date <= CURRENT_DATE + INTERVAL '30 days'`, tenantID).Scan(&s.ReviewDue)
	return s, nil
}

func (srv *server) handleGetPolicyStats(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	stats, err := srv.getPolicyStats(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fetch failed")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
