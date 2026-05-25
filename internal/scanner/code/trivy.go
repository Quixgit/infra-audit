package code

import (
	"encoding/json"
	"os/exec"
	"strings"

	"infra-audit/internal/model"
)

type trivyResult struct {
	Results []struct {
		Misconfigurations []struct {
			ID          string `json:"ID"`
			Title       string `json:"Title"`
			Description string `json:"Description"`
			Resolution  string `json:"Resolution"`
			Severity    string `json:"Severity"`
			CauseMetadata struct {
				Resource  string `json:"Resource"`
				Provider  string `json:"Provider"`
				StartLine int    `json:"StartLine"`
			} `json:"CauseMetadata"`
		} `json:"Misconfigurations"`
		Target string `json:"Target"`
	} `json:"Results"`
}

func RunTrivy(tfDir string) ([]model.CodeFinding, error) {
	cmd := exec.Command("trivy", "config",
		"--format", "json",
		"--quiet",
		tfDir,
	)
	out, _ := cmd.Output()

	var result trivyResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, nil
	}

	var findings []model.CodeFinding
	for _, r := range result.Results {
		for _, m := range r.Misconfigurations {
			findings = append(findings, model.CodeFinding{
				Tool:        "trivy",
				Category:    "Terraform",
				Severity:    strings.Title(strings.ToLower(m.Severity)),
				Title:       m.Title,
				Description: m.Description,
				File:        r.Target,
				Line:        m.CauseMetadata.StartLine,
				RuleID:      m.ID,
				Remediation: m.Resolution,
			})
		}
	}
	return findings, nil
}
