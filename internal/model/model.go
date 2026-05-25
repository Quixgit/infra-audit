package model

type Inventory struct {
	Client      string                 `json:"client"`
	Provider    string                 `json:"provider"`
	CollectedAt string                 `json:"collected_at"`
	Scope       []string               `json:"scope"`
	Resources   map[string]interface{} `json:"resources"`
	Errors      []string               `json:"errors,omitempty"`
}

type ReportMeta struct {
	Client           string   `json:"client"`
	ProjectName      string   `json:"project_name"`
	Provider         string   `json:"provider"`
	PreparedBy       string   `json:"prepared_by"`
	AuditorOrg       string   `json:"auditor_org"`
	AuditorAddress   string   `json:"auditor_address"`
	AuditorEmail     string   `json:"auditor_email"`
	AuditorWebsite   string   `json:"auditor_website"`
	AuditorPhone     string   `json:"auditor_phone"`
	Classification   string   `json:"classification"`
	AssessmentPeriod string   `json:"assessment_period"`
	GeneratedAt      string   `json:"generated_at"`
	ReportDate       string   `json:"report_date"`
	Version          string   `json:"version"`
	ArtifactBase     string   `json:"artifact_base"`
	LogoPath         string   `json:"logo_path"`
	WatermarkPath    string   `json:"watermark_path"`
	FooterBgPath     string   `json:"footer_bg_path"`
	Standards        []string `json:"standards"`
}

type Finding struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Severity           string   `json:"severity"`
	Status             string   `json:"status"`
	Category           string   `json:"category"`
	ResourceType       string   `json:"resource_type"`
	ResourceName       string   `json:"resource_name"`
	ResourceID         string   `json:"resource_id"`
	AffectedComponents []string `json:"affected_components"`
	Standard           string   `json:"standard"`
	ControlMapping     []string `json:"control_mapping"`
	Risk               string   `json:"risk"`
	BusinessImpact     string   `json:"business_impact"`
	Evidence           string   `json:"evidence"`
	Recommendation     string   `json:"recommendation"`
	Remediation        string   `json:"remediation"`
	Validation         string   `json:"validation"`
	Priority           string   `json:"priority"`
	Timeline           string   `json:"timeline"`
}

type PositiveFinding struct {
	Area     string `json:"area"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

type Report struct {
	Meta        ReportMeta        `json:"meta"`
	Inventory   Inventory         `json:"inventory"`
	Summary     string            `json:"summary"`
	Findings    []Finding         `json:"findings"`
	Positives   []PositiveFinding `json:"positive_findings"`
	Limitations []string          `json:"limitations"`
}
