package main

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ── Static framework / control definitions ────────────────────────────────────

type fwDef struct {
	Slug        string
	Name        string
	Version     string
	Description string
	Controls    []ctrlDef
}

type ctrlDef struct {
	ID          string
	Name        string
	Description string
}

// complianceFrameworks lists all supported frameworks.
// Order here determines display order on the frontend.
var complianceFrameworks = []fwDef{
	{
		Slug:        "soc2",
		Name:        "SOC 2 Type II",
		Version:     "2017",
		Description: "AICPA Trust Service Criteria for security, availability, and confidentiality",
		Controls: []ctrlDef{
			{"CC6.1", "Logical Access — Registration & Authorization", "Logical access security software, infrastructure, and architectures are implemented to protect information assets from security events."},
			{"CC6.2", "Logical Access — New User Registration", "Prior to issuing system credentials, new internal and external users are registered and authorized."},
			{"CC6.3", "Logical Access — Role-Based Access", "Access to data, software, and functions is authorized, modified, or removed based on roles."},
			{"CC6.6", "Logical Access — Network & Infrastructure Protection", "Controls prevent or detect unauthorized or malicious network and infrastructure access."},
			{"CC6.7", "Logical Access — Restricts Data Transmission", "Transmission, movement, and removal of information is restricted to authorized users and processes."},
			{"CC7.1", "System Operations — Manages Vulnerabilities", "Detection and monitoring procedures identify changes and vulnerabilities in configuration and software."},
			{"CC7.2", "System Operations — Monitors Infrastructure", "Infrastructure and software are monitored for anomalies indicating malicious acts, errors, or disasters."},
			{"CC7.3", "System Operations — Evaluates Security Events", "Security events are evaluated to determine whether unauthorized access or disclosure has occurred."},
			{"A1.1", "Availability — Manages Capacity", "Current processing capacity and use of system components are maintained, monitored, and evaluated."},
			{"A1.2", "Availability — Recovery & Continuity", "Environmental protections, software, data back-up processes, and recovery are authorized and maintained."},
		},
	},
	{
		Slug:        "iso27001",
		Name:        "ISO/IEC 27001",
		Version:     "2022",
		Description: "International standard for information security management systems (ISMS)",
		Controls: []ctrlDef{
			{"A.5.9", "Inventory of Information Assets", "An inventory of information and other associated assets shall be developed and maintained."},
			{"A.5.15", "Access Control", "Rules to control physical and logical access to information and associated assets shall be established and implemented."},
			{"A.5.16", "Identity Management", "The full life cycle of identities shall be managed."},
			{"A.5.30", "ICT Readiness for Business Continuity", "ICT readiness shall be planned, implemented, maintained and tested based on business continuity objectives."},
			{"A.8.8", "Management of Technical Vulnerabilities", "Information about technical vulnerabilities shall be obtained in a timely fashion, the organization's exposure evaluated, and appropriate measures taken."},
			{"A.8.12", "Data Leakage Prevention", "Data leakage prevention measures shall be applied to systems, networks and any other devices that process, store or transmit sensitive information."},
			{"A.8.15", "Logging", "Logs that record activities, exceptions, faults and other relevant events shall be produced, stored, protected and analysed."},
			{"A.8.16", "Monitoring Activities", "Networks, systems and applications shall be monitored for anomalous behaviour and appropriate actions taken."},
			{"A.8.20", "Networks Security", "Networks and network devices shall be secured, managed and controlled to protect information in systems and applications."},
			{"A.8.24", "Use of Cryptography", "Rules for the effective use of cryptography, including cryptographic key management, shall be defined and implemented."},
		},
	},
	{
		Slug:        "nist-csf",
		Name:        "NIST Cybersecurity Framework",
		Version:     "1.1",
		Description: "Framework for improving critical infrastructure cybersecurity",
		Controls: []ctrlDef{
			{"ID.AM", "Asset Management", "Data, personnel, devices, systems, and facilities that enable business purposes are identified and managed consistent with their relative importance."},
			{"PR.AC", "Identity Management & Access Control", "Access to physical and logical assets and associated facilities is limited to authorized users, processes, and devices."},
			{"PR.DS", "Data Security", "Information and records are managed consistent with the organization's risk strategy to protect confidentiality, integrity, and availability."},
			{"PR.IP", "Information Protection Processes", "Security policies, processes, and procedures are maintained and used to manage protection of information systems and assets."},
			{"PR.PT", "Protective Technology", "Technical security solutions are managed to ensure the security and resilience of systems and assets."},
			{"DE.CM", "Security Continuous Monitoring", "The information system and assets are monitored to identify cybersecurity events and verify the effectiveness of protective measures."},
			{"RS.MI", "Mitigation", "Activities are performed to prevent expansion of an event, mitigate its effects, and resolve the incident."},
		},
	},
	{
		Slug:        "cis-v8",
		Name:        "CIS Controls",
		Version:     "v8",
		Description: "Center for Internet Security Controls — prioritized set of actions for cyber defense",
		Controls: []ctrlDef{
			{"CIS-1", "Inventory and Control of Enterprise Assets", "Actively manage all enterprise assets connected to the infrastructure to accurately know the totality of assets."},
			{"CIS-3", "Data Protection", "Develop processes and technical controls to identify, classify, securely handle, retain, and dispose of data."},
			{"CIS-4", "Secure Configuration", "Establish and maintain the secure configuration of enterprise assets and software."},
			{"CIS-6", "Access Control Management", "Use processes and tools to create, assign, manage, and revoke access credentials and privileges for all accounts."},
			{"CIS-7", "Continuous Vulnerability Management", "Develop a plan to continuously assess and track vulnerabilities on all enterprise assets."},
			{"CIS-11", "Data Recovery", "Establish and maintain data recovery practices sufficient to restore in-scope enterprise assets to a pre-incident and trusted state."},
			{"CIS-12", "Network Infrastructure Management", "Establish and maintain the secure management of network infrastructure."},
			{"CIS-13", "Network Monitoring and Defense", "Operate processes and tooling to establish and maintain comprehensive network monitoring and defense against security threats."},
		},
	},
	{
		Slug:        "hipaa",
		Name:        "HIPAA Security Rule",
		Version:     "2013",
		Description: "US Health Insurance Portability and Accountability Act — Technical and Administrative Safeguards for ePHI",
		Controls: []ctrlDef{
			{"§164.308(a)(1)", "Security Management Process", "Implement policies and procedures to prevent, detect, contain, and correct security violations including risk analysis and risk management."},
			{"§164.308(a)(3)", "Workforce Security", "Implement policies and procedures to ensure that all members of its workforce have appropriate access to ePHI and prevent those who do not have access from obtaining it."},
			{"§164.308(a)(5)", "Security Awareness and Training", "Implement a security awareness and training program for all members of the workforce."},
			{"§164.308(a)(7)", "Contingency Plan", "Establish and implement policies and procedures for responding to an emergency including data backup and disaster recovery plans."},
			{"§164.312(a)(1)", "Access Control", "Implement technical policies and procedures for electronic information systems that maintain ePHI to allow access only to authorized persons or software programs."},
			{"§164.312(b)", "Audit Controls", "Implement hardware, software, and/or procedural mechanisms that record and examine activity in information systems that contain or use ePHI."},
			{"§164.312(c)(1)", "Integrity Controls", "Implement policies and procedures to protect ePHI from improper alteration or destruction."},
			{"§164.312(d)", "Person/Entity Authentication", "Implement procedures to verify that a person or entity seeking access to ePHI is the one claimed."},
			{"§164.312(e)(1)", "Transmission Security", "Implement technical security measures to guard against unauthorized access to ePHI that is being transmitted over an electronic communications network."},
		},
	},
	{
		Slug:        "pci-dss",
		Name:        "PCI DSS",
		Version:     "v4.0",
		Description: "Payment Card Industry Data Security Standard — requirements for organizations that handle cardholder data",
		Controls: []ctrlDef{
			{"Req 1", "Network Security Controls", "Install and maintain network security controls to protect the cardholder data environment."},
			{"Req 2", "Secure Configurations", "Apply secure configurations to all system components to protect against known vulnerabilities."},
			{"Req 3", "Protect Stored Account Data", "Protect stored account data using strong cryptography and minimization practices."},
			{"Req 4", "Protect Data in Transit", "Protect cardholder data with strong cryptography during transmission over open, public networks."},
			{"Req 6", "Secure Systems and Software", "Develop and maintain secure systems and software by identifying and addressing vulnerabilities."},
			{"Req 7", "Restrict Access", "Restrict access to system components and cardholder data by business need to know."},
			{"Req 8", "Identify and Authenticate Users", "Identify users and authenticate access to system components using multi-factor authentication and strong credentials."},
			{"Req 10", "Log and Monitor All Access", "Log and monitor all access to network resources and cardholder data."},
			{"Req 11", "Test Security Regularly", "Test security of systems and networks regularly using vulnerability scanning and penetration testing."},
		},
	},
	{
		Slug:        "gdpr",
		Name:        "GDPR",
		Version:     "2018",
		Description: "EU General Data Protection Regulation — technical and organisational security measures for personal data processing",
		Controls: []ctrlDef{
			{"Art.5", "Principles of Processing", "Personal data shall be processed lawfully, fairly, and in a transparent manner — including integrity and confidentiality."},
			{"Art.25", "Data Protection by Design", "Implement appropriate technical and organisational measures designed to implement data protection principles effectively."},
			{"Art.32", "Security of Processing", "Implement appropriate technical and organisational measures to ensure a level of security appropriate to the risk including encryption and pseudonymisation."},
			{"Art.33", "Breach Notification", "Notify the supervisory authority of a personal data breach within 72 hours of becoming aware of it."},
			{"Art.35", "Data Protection Impact Assessment", "Carry out a DPIA prior to processing likely to result in high risk to individuals' rights and freedoms."},
		},
	},
}

// ── Finding → control mapping ─────────────────────────────────────────────────

var categoryControlMap = map[string]map[string][]string{
	"hipaa": {
		"Secrets Management":                 {"§164.312(a)(1)", "§164.312(e)(1)"},
		"Identity and Account Security":      {"§164.312(a)(1)", "§164.312(d)"},
		"Network Security":                   {"§164.312(a)(1)", "§164.312(e)(1)"},
		"Database Network Security":          {"§164.312(a)(1)"},
		"Database Access Control":            {"§164.312(a)(1)", "§164.312(d)"},
		"Availability and Resilience":        {"§164.308(a)(7)"},
		"Backup and Recovery":                {"§164.308(a)(7)"},
		"Patch and Vulnerability Management": {"§164.308(a)(1)"},
		"Asset Management":                   {"§164.308(a)(1)"},
		"Kubernetes Security":                {"§164.312(a)(1)", "§164.308(a)(1)"},
		"Transport Security":                 {"§164.312(e)(1)"},
		"Monitoring and Alerting":            {"§164.312(b)"},
		"Monitoring and Telemetry":           {"§164.312(b)"},
		"Architecture and Resilience":        {"§164.308(a)(7)"},
		"Application Security Configuration": {"§164.312(a)(1)"},
		"Identity and Data Access":           {"§164.312(a)(1)", "§164.312(d)"},
		"Availability and DNS Resilience":    {"§164.308(a)(7)"},
	},
	"pci-dss": {
		"Secrets Management":                 {"Req 3", "Req 4"},
		"Identity and Account Security":      {"Req 7", "Req 8"},
		"Network Security":                   {"Req 1", "Req 2"},
		"Database Network Security":          {"Req 1", "Req 7"},
		"Database Access Control":            {"Req 7", "Req 8"},
		"Availability and Resilience":        {"Req 2"},
		"Backup and Recovery":                {"Req 2"},
		"Patch and Vulnerability Management": {"Req 6", "Req 11"},
		"Asset Management":                   {"Req 2"},
		"Kubernetes Security":                {"Req 1", "Req 2", "Req 6"},
		"Transport Security":                 {"Req 4"},
		"Monitoring and Alerting":            {"Req 10"},
		"Monitoring and Telemetry":           {"Req 10"},
		"Architecture and Resilience":        {"Req 2"},
		"Application Security Configuration": {"Req 6"},
		"Identity and Data Access":           {"Req 7", "Req 8"},
		"Availability and DNS Resilience":    {"Req 2"},
	},
	"gdpr": {
		"Secrets Management":                 {"Art.32", "Art.33"},
		"Identity and Account Security":      {"Art.32"},
		"Network Security":                   {"Art.32"},
		"Database Network Security":          {"Art.32"},
		"Database Access Control":            {"Art.25", "Art.32"},
		"Availability and Resilience":        {"Art.32"},
		"Backup and Recovery":                {"Art.32"},
		"Patch and Vulnerability Management": {"Art.32", "Art.35"},
		"Asset Management":                   {"Art.5"},
		"Kubernetes Security":                {"Art.32"},
		"Transport Security":                 {"Art.32"},
		"Monitoring and Alerting":            {"Art.33"},
		"Monitoring and Telemetry":           {"Art.33"},
		"Architecture and Resilience":        {"Art.32"},
		"Application Security Configuration": {"Art.25", "Art.32"},
		"Identity and Data Access":           {"Art.25", "Art.32"},
		"Availability and DNS Resilience":    {"Art.32"},
	},
	"soc2": {
		"Secrets Management":                 {"CC6.7", "CC7.1"},
		"Identity and Account Security":      {"CC6.1", "CC6.2"},
		"Network Security":                   {"CC6.6", "CC6.7"},
		"Database Network Security":          {"CC6.6"},
		"Database Access Control":            {"CC6.1", "CC6.3"},
		"Availability and Resilience":        {"A1.1", "A1.2"},
		"Backup and Recovery":                {"A1.2"},
		"Patch and Vulnerability Management": {"CC7.1"},
		"Asset Management":                   {},
		"Kubernetes Security":                {"CC6.6", "CC7.1"},
		"Transport Security":                 {"CC6.7"},
		"Monitoring and Alerting":            {"CC7.2", "CC7.3"},
		"Monitoring and Telemetry":           {"CC7.2"},
		"Architecture and Resilience":        {"A1.1"},
		"Application Security Configuration": {"CC6.6"},
		"Identity and Data Access":           {"CC6.1", "CC6.3"},
		"Availability and DNS Resilience":    {"A1.1"},
	},
	"iso27001": {
		"Secrets Management":                 {"A.8.12", "A.5.15"},
		"Identity and Account Security":      {"A.5.16", "A.5.15"},
		"Network Security":                   {"A.8.20"},
		"Database Network Security":          {"A.8.20"},
		"Database Access Control":            {"A.5.15"},
		"Availability and Resilience":        {"A.5.30"},
		"Backup and Recovery":                {"A.5.30"},
		"Patch and Vulnerability Management": {"A.8.8"},
		"Asset Management":                   {"A.5.9"},
		"Kubernetes Security":                {"A.8.20", "A.8.8"},
		"Transport Security":                 {"A.8.24"},
		"Monitoring and Alerting":            {"A.8.16"},
		"Monitoring and Telemetry":           {"A.8.15"},
		"Architecture and Resilience":        {"A.5.30"},
		"Application Security Configuration": {"A.8.20"},
		"Identity and Data Access":           {"A.5.15"},
		"Availability and DNS Resilience":    {"A.5.30"},
	},
	"nist-csf": {
		"Secrets Management":                 {"PR.DS", "RS.MI"},
		"Identity and Account Security":      {"PR.AC"},
		"Network Security":                   {"PR.AC", "PR.PT"},
		"Database Network Security":          {"PR.PT"},
		"Database Access Control":            {"PR.AC"},
		"Availability and Resilience":        {"PR.IP", "PR.PT"},
		"Backup and Recovery":                {"PR.IP"},
		"Patch and Vulnerability Management": {"PR.IP", "RS.MI"},
		"Asset Management":                   {"ID.AM"},
		"Kubernetes Security":                {"PR.AC", "PR.PT"},
		"Transport Security":                 {"PR.DS"},
		"Monitoring and Alerting":            {"DE.CM"},
		"Monitoring and Telemetry":           {"DE.CM"},
		"Architecture and Resilience":        {"PR.PT"},
		"Application Security Configuration": {"PR.AC"},
		"Identity and Data Access":           {"PR.AC"},
		"Availability and DNS Resilience":    {"PR.PT"},
	},
	"cis-v8": {
		"Secrets Management":                 {"CIS-3"},
		"Identity and Account Security":      {"CIS-6"},
		"Network Security":                   {"CIS-4", "CIS-12"},
		"Database Network Security":          {"CIS-4", "CIS-12"},
		"Database Access Control":            {"CIS-6"},
		"Availability and Resilience":        {"CIS-11"},
		"Backup and Recovery":                {"CIS-11"},
		"Patch and Vulnerability Management": {"CIS-7"},
		"Asset Management":                   {"CIS-1"},
		"Kubernetes Security":                {"CIS-4", "CIS-12"},
		"Transport Security":                 {"CIS-3"},
		"Monitoring and Alerting":            {"CIS-13"},
		"Monitoring and Telemetry":           {"CIS-13"},
		"Architecture and Resilience":        {"CIS-11"},
		"Application Security Configuration": {"CIS-4"},
		"Identity and Data Access":           {"CIS-6"},
		"Availability and DNS Resilience":    {"CIS-11"},
	},
}

// toolControlMap maps tool names (gitleaks, semgrep, tf) to control IDs per framework.
var toolControlMap = map[string]map[string][]string{
	"soc2": {
		"gitleaks":   {"CC6.7", "CC7.1"},
		"trufflehog": {"CC6.7", "CC7.1"},
		"semgrep":    {"CC7.1"},
		"tf":         {"CC6.6"},
	},
	"iso27001": {
		"gitleaks":   {"A.8.12"},
		"trufflehog": {"A.8.12"},
		"semgrep":    {"A.8.8"},
		"tf":         {"A.8.20"},
	},
	"nist-csf": {
		"gitleaks":   {"PR.DS"},
		"trufflehog": {"PR.DS"},
		"semgrep":    {"PR.IP"},
		"tf":         {"PR.PT"},
	},
	"cis-v8": {
		"gitleaks":   {"CIS-3"},
		"trufflehog": {"CIS-3"},
		"semgrep":    {"CIS-7"},
		"tf":         {"CIS-4"},
	},
	"hipaa": {
		"gitleaks":   {"§164.312(a)(1)"},
		"trufflehog": {"§164.312(a)(1)"},
		"semgrep":    {"§164.308(a)(1)"},
		"tf":         {"§164.312(a)(1)"},
	},
	"pci-dss": {
		"gitleaks":   {"Req 3"},
		"trufflehog": {"Req 3"},
		"semgrep":    {"Req 6"},
		"tf":         {"Req 1", "Req 2"},
	},
	"gdpr": {
		"gitleaks":   {"Art.32"},
		"trufflehog": {"Art.32"},
		"semgrep":    {"Art.32"},
		"tf":         {"Art.25"},
	},
}

func findingControlIDs(af AggregatedFinding, frameworkSlug string) []string {
	if af.Category != "" {
		if m, ok := categoryControlMap[frameworkSlug]; ok {
			if ids, ok := m[af.Category]; ok {
				return ids
			}
		}
	}
	if af.Tool != "" {
		tool := strings.ToLower(af.Tool)
		if m, ok := toolControlMap[frameworkSlug]; ok {
			if ids, ok := m[tool]; ok {
				return ids
			}
		}
	}
	if af.Source == "tf_findings" {
		if m, ok := toolControlMap[frameworkSlug]; ok {
			if ids, ok := m["tf"]; ok {
				return ids
			}
		}
	}
	return nil
}

// ── API response types ────────────────────────────────────────────────────────

type ComplianceFrameworkResponse struct {
	Slug        string                      `json:"slug"`
	Name        string                      `json:"name"`
	Version     string                      `json:"version"`
	Description string                      `json:"description"`
	Score       int                         `json:"score"`
	MetCount    int                         `json:"met_count"`
	TotalCount  int                         `json:"total_count"`
	Controls    []ComplianceControlResponse `json:"controls,omitempty"`
}

type ComplianceControlResponse struct {
	CtrlID       string              `json:"ctrl_id"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	Status       string              `json:"status"` // met, partial, not_met
	FindingCount int                 `json:"finding_count"`
	OpenCount    int                 `json:"open_count"`
	Findings     []AggregatedFinding `json:"findings,omitempty"`
}

// ── Build logic ───────────────────────────────────────────────────────────────

func buildComplianceFramework(slug string, allFindings []AggregatedFinding, includeFindingsList bool) ComplianceFrameworkResponse {
	var fw *fwDef
	for i := range complianceFrameworks {
		if complianceFrameworks[i].Slug == slug {
			fw = &complianceFrameworks[i]
			break
		}
	}
	if fw == nil {
		return ComplianceFrameworkResponse{}
	}

	type ctrlState struct {
		def          ctrlDef
		findingCount int
		openCount    int
		findings     []AggregatedFinding
	}

	stateMap := make(map[string]*ctrlState, len(fw.Controls))
	orderedIDs := make([]string, 0, len(fw.Controls))
	for _, c := range fw.Controls {
		cCopy := c
		stateMap[c.ID] = &ctrlState{def: cCopy}
		orderedIDs = append(orderedIDs, c.ID)
	}

	for _, af := range allFindings {
		ids := findingControlIDs(af, slug)
		for _, id := range ids {
			st, ok := stateMap[id]
			if !ok {
				continue
			}
			st.findingCount++
			if af.Status == "open" || af.Status == "in_progress" {
				st.openCount++
			}
			if includeFindingsList {
				st.findings = append(st.findings, af)
			}
		}
	}

	metCount := 0
	partialCount := 0
	notAssessedCount := 0
	var controls []ComplianceControlResponse
	for _, id := range orderedIDs {
		st := stateMap[id]
		// Controls are assessed only when the scanner produced at least one finding
		// that maps to them. Controls with zero findings are "not_assessed" — the
		// automated scan simply does not cover that area (e.g. user-registration
		// processes, manual monitoring procedures). They are shown neutrally and
		// excluded from the score denominator so they don't unfairly drag down the %.
		var status string
		switch {
		case st.findingCount == 0:
			// Scan does not cover this control → show as not assessed, skip from score
			status = "not_assessed"
			notAssessedCount++
		case st.openCount == 0:
			// Audited; all issues resolved → fully compliant
			status = "met"
			metCount++
		case st.openCount < st.findingCount:
			// Some findings still open
			status = "partial"
			partialCount++
		default:
			// All findings are open
			status = "not_met"
		}
		ctrl := ComplianceControlResponse{
			CtrlID:       st.def.ID,
			Name:         st.def.Name,
			Description:  st.def.Description,
			Status:       status,
			FindingCount: st.findingCount,
			OpenCount:    st.openCount,
		}
		if includeFindingsList {
			ctrl.Findings = st.findings
		}
		controls = append(controls, ctrl)
	}

	// Score is calculated only over assessed controls (those with ≥1 finding).
	// "not_assessed" controls are excluded so they don't skew the result.
	total := len(fw.Controls)
	assessed := total - notAssessedCount
	score := 0
	if assessed > 0 {
		score = (metCount*100 + partialCount*50) / assessed
	}

	return ComplianceFrameworkResponse{
		Slug:        fw.Slug,
		Name:        fw.Name,
		Version:     fw.Version,
		Description: fw.Description,
		Score:       score,
		MetCount:    metCount,
		TotalCount:  assessed, // show assessed controls, not total (clearer for user)
		Controls:    controls,
	}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (srv *server) handleGetComplianceFrameworks(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)

	allFindings, err := srv.getAllAggregatedFindings(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	result := make([]ComplianceFrameworkResponse, 0, len(complianceFrameworks))
	for _, fw := range complianceFrameworks {
		result = append(result, buildComplianceFramework(fw.Slug, allFindings, false))
	}

	writeJSON(w, http.StatusOK, result)
}

func (srv *server) handleGetComplianceFramework(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxTenantID).(string)
	slug := chi.URLParam(r, "slug")

	found := false
	for _, fw := range complianceFrameworks {
		if fw.Slug == slug {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "framework not found")
		return
	}

	allFindings, err := srv.getAllAggregatedFindings(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db error")
		return
	}

	writeJSON(w, http.StatusOK, buildComplianceFramework(slug, allFindings, true))
}
