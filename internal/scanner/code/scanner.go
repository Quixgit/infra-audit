package code

import (
	"fmt"
	"os"

	"infra-audit/internal/model"
)

func Scan(repoPath string) ([]model.CodeFinding, []model.CodeFinding, Stack) {
	stack := DetectStack(repoPath)

	var codeFindings []model.CodeFinding
	var tfFindings []model.CodeFinding

	gf, err := RunGitleaks(repoPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "WARNING: gitleaks:", err)
	}
	codeFindings = append(codeFindings, gf...)

	sf, err := RunSemgrep(repoPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "WARNING: semgrep:", err)
	}
	codeFindings = append(codeFindings, sf...)

	for _, dir := range stack.NodeDirs {
		nf, err := RunNpmAudit(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: npm audit:", err)
		}
		codeFindings = append(codeFindings, nf...)
	}

	if stack.HasTerraform {
		hf, err := RunHCLScan(repoPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: hclscan:", err)
		}
		tfFindings = append(tfFindings, hf...)
	}

	if stack.HasTerraform {
		for _, dir := range stack.TerraformDirs {
			tf, err := RunTrivy(dir)
			if err != nil {
				fmt.Fprintln(os.Stderr, "WARNING: trivy:", err)
			}
			tfFindings = append(tfFindings, tf...)
		}
	}

	return codeFindings, tfFindings, stack
}
