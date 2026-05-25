package code

import (
	"encoding/json"
	"os/exec"

	"infra-audit/internal/model"
)

type npmAuditResult struct {
	Vulnerabilities map[string]struct {
		Name     string `json:"name"`
		Severity string `json:"severity"`
		Via      []interface{} `json:"via"`
		FixAvailable interface{} `json:"fixAvailable"`
	} `json:"vulnerabilities"`
}

func RunNpmAudit(dir string) ([]model.CodeFinding, error) {
	cmd := exec.Command("npm", "audit", "--json")
	cmd.Dir = dir
	out, _ := cmd.Output()

	var result npmAuditResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, nil
	}

	var findings []model.CodeFinding
	for _, v := range result.Vulnerabilities {
		findings = append(findings, model.CodeFinding{
			Tool:        "npm-audit",
			Category:    "Dependency",
			Severity:    normalizeSeverity(v.Severity),
			Title:       "Vulnerable dependency: " + v.Name,
			File:        dir + "/package.json",
			Remediation: "Run npm audit fix or update the affected package.",
		})
	}
	return findings, nil
}
