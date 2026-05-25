package model

type CodeFinding struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	RuleID      string `json:"rule_id"`
	Tool        string `json:"tool"`
	Remediation string `json:"remediation"`
	CVE         string `json:"cve,omitempty"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
}

type CodeReport struct {
	Meta      ReportMeta
	RepoPath  string
	Stack     []string
	Findings  []CodeFinding
	TFFindings []CodeFinding
	Summary   string
}
