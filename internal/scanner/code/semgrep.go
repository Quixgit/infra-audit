package code

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"infra-audit/internal/model"
)

type semgrepResult struct {
	Results []struct {
		CheckID string `json:"check_id"`
		Path    string `json:"path"`
		Start   struct {
			Line int `json:"line"`
		} `json:"start"`
		Extra struct {
			Message  string `json:"message"`
			Severity string `json:"severity"`
			Metadata struct {
				Category string `json:"category"`
			} `json:"metadata"`
		} `json:"extra"`
	} `json:"results"`
}

func RunSemgrep(repoPath string) ([]model.CodeFinding, error) {
	if _, err := exec.LookPath("semgrep"); err != nil {
		return nil, fmt.Errorf("semgrep not found in PATH")
	}

	tmp, err := os.CreateTemp("", "semgrep-*.json")
	if err != nil {
		return nil, err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command("semgrep",
		"--config", "auto",
		"--json",
		"--output", tmp.Name(),
		"--no-rewrite-rule-ids",
		"--quiet",
		repoPath,
	)
	cmd.Run()

	data, err := readFileBytes(tmp.Name())
	if err != nil {
		return nil, nil
	}

	var result semgrepResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil
	}

	// Deduplicate: same rule + same file = one finding
	seen := map[string]bool{}
	var findings []model.CodeFinding
	for _, r := range result.Results {
		key := r.CheckID + "|" + r.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		sev := normalizeSeverity(r.Extra.Severity)
		findings = append(findings, model.CodeFinding{
			Tool:        "semgrep",
			Category:    r.Extra.Metadata.Category,
			Severity:    sev,
			Title:       r.Extra.Message,
			File:        trimRepoPath(r.Path, repoPath),
			Line:        r.Start.Line,
			RuleID:      r.CheckID,
			Remediation: "Review and fix according to the rule: " + r.CheckID,
		})
	}
	return findings, nil
}

func normalizeSeverity(s string) string {
	switch s {
	case "ERROR":
		return "High"
	case "WARNING":
		return "Medium"
	case "INFO":
		return "Low"
	default:
		return "Info"
	}
}
