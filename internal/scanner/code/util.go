package code

import (
	"os"
	"strings"
)

func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func trimRepoPath(path, repoPath string) string {
	if strings.HasPrefix(path, repoPath) {
		p := path[len(repoPath):]
		if len(p) > 0 && (p[0] == '/' || p[0] == '\\') {
			p = p[1:]
		}
		return p
	}
	return path
}
