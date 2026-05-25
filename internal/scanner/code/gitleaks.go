package code

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"infra-audit/internal/model"
)

type gitleaksResult struct {
	Description string `json:"Description"`
	File        string `json:"File"`
	Line        int    `json:"StartLine"`
	RuleID      string `json:"RuleID"`
	Secret      string `json:"Secret"`
	Severity    string `json:"Severity"`
}

func RunGitleaks(repoPath string) ([]model.CodeFinding, error) {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		return nil, fmt.Errorf("gitleaks not found in PATH")
	}

	tmp, err := os.CreateTemp("", "gitleaks-*.json")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command("gitleaks", "detect",
		"--source", repoPath,
		"--report-format", "json",
		"--report-path", tmp.Name(),
		"--no-git",
		"--exit-code", "0",
	)
	cmd.Run()

	data, err := readFileBytes(tmp.Name())
	if err != nil {
		return nil, nil
	}

	var results []gitleaksResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, nil
	}

	var findings []model.CodeFinding
	for _, r := range results {
		sev := "High"
		if r.Severity != "" {
			sev = r.Severity
		}
		// .env files with secrets are Critical
		if strings.Contains(r.File, ".env") {
			sev = "Critical"
		}
		findings = append(findings, model.CodeFinding{
			Tool:        "gitleaks",
			Category:    "Secrets",
			Severity:    sev,
			Title:       "Secret detected: " + r.Description,
			File:        trimRepoPath(r.File, repoPath),
			Line:        r.Line,
			RuleID:      r.RuleID,
			Remediation: "Remove the secret from the codebase, rotate credentials, and use environment variables or a secrets manager.",
		})
	}
	return findings, nil
}
