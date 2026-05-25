package code

import (
	"os"
	"path/filepath"
)

type Stack struct {
	HasNode      bool
	HasTypeScript bool
	HasGo        bool
	HasPython    bool
	HasTerraform bool
	HasDocker    bool
	NodeDirs     []string
	TerraformDirs []string
	DockerFiles  []string
}

func DetectStack(repoPath string) Stack {
	var s Stack

	filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and node_modules
		if info.IsDir() {
			base := info.Name()
			if base == "node_modules" || base == ".git" || base == ".terraform" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, _ := filepath.Rel(repoPath, path)
		dir := filepath.Dir(path)

		switch info.Name() {
		case "package.json":
			s.HasNode = true
			s.NodeDirs = appendUniq(s.NodeDirs, dir)
		case "tsconfig.json":
			s.HasTypeScript = true
		case "go.mod":
			s.HasGo = true
		case "requirements.txt", "pyproject.toml", "setup.py":
			s.HasPython = true
		case "Dockerfile":
			s.HasDocker = true
			s.DockerFiles = appendUniq(s.DockerFiles, rel)
		}

		if filepath.Ext(path) == ".tf" {
			s.HasTerraform = true
			s.TerraformDirs = appendUniq(s.TerraformDirs, dir)
		}

		return nil
	})

	return s
}

func appendUniq(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
