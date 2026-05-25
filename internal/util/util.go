package util

import "strings"

// SafeName converts an arbitrary string into a safe lowercase filename token.
// Non-alphanumeric characters are replaced with underscores; consecutive
// underscores are collapsed; leading/trailing underscores are trimmed.
func SafeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "client_security_audit"
	}

	var b strings.Builder
	lastUnderscore := false

	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}

	return strings.Trim(b.String(), "_")
}
