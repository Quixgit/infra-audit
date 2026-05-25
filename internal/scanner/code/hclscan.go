package code

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"infra-audit/internal/model"
)

type hclRule struct {
	ID          string
	Severity    string
	Title       string
	Description string
	Remediation string
	Match       func(line string) bool
}

var hclRules = []hclRule{
	{
		ID:          "TF-001",
		Severity:    "High",
		Title:       "Hardcoded resource ID in Terraform code",
		Description: "A resource ID (UUID) is hardcoded directly in the Terraform code instead of using a variable or data source.",
		Remediation: "Replace hardcoded IDs with variables or data sources to avoid environment coupling and accidental exposure.",
		Match: func(line string) bool {
			re := regexp.MustCompile(`=\s*"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"`)
			return re.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "#")
		},
	},
	{
		ID:          "TF-002",
		Severity:    "Medium",
		Title:       "Default database superuser account used",
		Description: "The database is configured with the default superuser account 'doadmin' instead of a dedicated application user with least-privilege access.",
		Remediation: "Create a dedicated database user with only the permissions required by the application.",
		Match: func(line string) bool {
			return strings.Contains(line, `username`) &&
				strings.Contains(line, `"doadmin"`) &&
				!strings.HasPrefix(strings.TrimSpace(line), "#")
		},
	},
	{
		ID:          "TF-003",
		Severity:    "Medium",
		Title:       "Autodeploy enabled without approval gate",
		Description: "The application is configured with autodeploy=true, meaning any push to the branch triggers an immediate production deployment without manual approval.",
		Remediation: "Disable autodeploy for production environments or implement a CI/CD approval step before deployment.",
		Match: func(line string) bool {
			return strings.Contains(line, "autodeploy") &&
				strings.Contains(line, "true") &&
				!strings.HasPrefix(strings.TrimSpace(line), "#")
		},
	},
	{
		ID:          "TF-004",
		Severity:    "Low",
		Title:       "Sensitive variable passed directly to application module",
		Description: "Secrets such as API keys, tokens, and passwords are passed as plain Terraform variables to application modules. If state or plan files are exposed, these values may be visible.",
		Remediation: "Use a secrets manager (e.g. Vault, DO Secrets) and inject secrets at runtime rather than passing them through Terraform variables.",
		Match: func(line string) bool {
			sensitiveKeys := []string{
				"SECRET", "PASSWORD", "API_KEY", "AUTH_TOKEN",
				"PRIVATE_KEY", "ACCESS_KEY", "JWT",
			}
			upper := strings.ToUpper(line)
			for _, k := range sensitiveKeys {
				if strings.Contains(upper, k) &&
					strings.Contains(line, "var.") &&
					!strings.HasPrefix(strings.TrimSpace(line), "#") {
					return true
				}
			}
			return false
		},
	},
	{
		ID:          "TF-005",
		Severity:    "Low",
		Title:       "Terraform code stored in the same repository as application code",
		Description: "Infrastructure-as-code (Terraform) is stored in the same repository as application source code. This increases the blast radius of a repository compromise and complicates access control.",
		Remediation: "Move Terraform code to a dedicated infrastructure repository with separate access controls, branch protection, and CI/CD pipelines.",
		Match: func(line string) bool {
			return false // detected at directory level, not line level
		},
	},
	{
		ID:          "TF-006",
		Severity:    "Info",
		Title:       "Terraform state managed via Terraform Cloud",
		Description: "State is stored in Terraform Cloud. Ensure the organization has MFA enforced, API tokens are rotated regularly, and workspace access is restricted to authorised team members.",
		Remediation: "Review Terraform Cloud organization settings: enforce MFA, restrict workspace access, and rotate team tokens periodically.",
		Match: func(line string) bool {
			return strings.Contains(line, "cloud {") &&
				!strings.HasPrefix(strings.TrimSpace(line), "#")
		},
	},
	{
		ID:          "TF-007",
		Severity:    "Medium",
		Title:       "Commented-out backend configuration left in code",
		Description: "A previous backend configuration (S3/Spaces) is commented out in the code. Commented infrastructure code can cause confusion about the active state backend and may be accidentally re-enabled.",
		Remediation: "Remove commented-out backend configurations or document clearly why they are retained.",
		Match: func(line string) bool {
			trimmed := strings.TrimSpace(line)
			return strings.HasPrefix(trimmed, "#") &&
				(strings.Contains(line, "backend") || strings.Contains(line, "bucket") || strings.Contains(line, "tfstate"))
		},
	},
}

func RunHCLScan(repoPath string) ([]model.CodeFinding, error) {
	var findings []model.CodeFinding
	seen := map[string]bool{}

	// TF-005: terraform in same repo as code — detect once
	hasTF := false
	hasCode := false
	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == "node_modules" || info.Name() == ".git") {
			return filepath.SkipDir
		}
		if filepath.Ext(path) == ".tf" {
			hasTF = true
		}
		if info.Name() == "package.json" || info.Name() == "go.mod" {
			hasCode = true
		}
		return nil
	})
	if hasTF && hasCode {
		findings = append(findings, model.CodeFinding{
			Tool:        "hclscan",
			Category:    "Infrastructure",
			Severity:    "Low",
			Title:       hclRules[4].Title,
			Description: hclRules[4].Description,
			File:        "terraform/",
			RuleID:      "TF-005",
			Remediation: hclRules[4].Remediation,
		})
	}

	// Scan all .tf files line by line
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "node_modules" || info.Name() == ".git" || info.Name() == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".tf" {
			return nil
		}

		rel, _ := filepath.Rel(repoPath, path)
		scanTFFile(path, rel, &findings, seen)
		return nil
	})

	return findings, err
}

func scanTFFile(path, rel string, findings *[]model.CodeFinding, seen map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, rule := range hclRules {
			if rule.ID == "TF-005" {
				continue // handled separately
			}
			if !rule.Match(line) {
				continue
			}
			// Deduplicate TF-004 and TF-007 per file
			key := fmt.Sprintf("%s:%s", rule.ID, rel)
			if (rule.ID == "TF-004" || rule.ID == "TF-007") && seen[key] {
				continue
			}
			seen[key] = true

			*findings = append(*findings, model.CodeFinding{
				Tool:        "hclscan",
				Category:    "Infrastructure",
				Severity:    rule.Severity,
				Title:       rule.Title,
				Description: rule.Description,
				File:        rel,
				Line:        lineNum,
				RuleID:      rule.ID,
				Remediation: rule.Remediation,
			})
		}
	}
}
