package rules

import (
	"fmt"
	"net"
	"strings"

	"infra-audit/internal/model"
)

func checkSpacesLite(inv model.Inventory, add func(model.Finding)) {
	for _, bucket := range arr(inv.Resources["spaces"]) {
		name := str(bucket, "name")
		if name == "" {
			continue
		}

		checks := obj(bucket["checks"])
		sensitive := boolVal(checks["sensitive_bucket_name"])

		if boolVal(checks["public_bucket_listing"]) {
			add(model.Finding{
				Severity:       "High",
				Category:       "Object Storage Access Control",
				Title:          "Spaces bucket listing appears publicly accessible",
				ResourceType:   "spaces_bucket",
				ResourceName:   name,
				Standard:       "ISO 27001 A.5.15/A.8.3 / NIST PR.AA / CIS Control 3",
				Risk:           "The bucket listing responded to an unauthenticated request. Public bucket listing can expose object names and may reveal sensitive data structure even when individual files are not directly reviewed.",
				BusinessImpact: "Object names, paths, customer file names, logs, backups, or state files may become visible to unauthenticated users.",
				Evidence:       fmt.Sprintf("unauthenticated bucket listing status=%v", checks["public_bucket_listing_status"]),
				Recommendation: "Disable public bucket listing unless this bucket is intentionally public.",
				Remediation:    "Review bucket permissions and remove public list/read access. Use signed URLs or a controlled CDN path for public delivery.",
				Validation:     "Re-run the scan and confirm public_bucket_listing=false.",
				Timeline:       "Short term / within 1 week",
			})
		}

		if sensitive && boolVal(checks["manual_validation_required"]) {
			add(model.Finding{
				Severity:       "Low",
				Category:       "Object Storage Configuration Review",
				Title:          "Sensitive Spaces bucket requires authenticated security review",
				ResourceType:   "spaces_bucket",
				ResourceName:   name,
				Standard:       "ISO 27001 A.5.15/A.8.3/A.5.33 / NIST PR.AA / CIS Control 3",
				Risk:           "The bucket name suggests it may contain sensitive or operational data. Without a Spaces access key, ACL, bucket policy, CORS, lifecycle and versioning settings cannot be verified automatically.",
				BusinessImpact: "If the bucket stores Terraform state, uploads, logs, backups or private files, incorrect permissions or missing recovery controls could expose data or make recovery harder.",
				Evidence:       "public probe completed, but authenticated Spaces configuration checks were not available.",
				Recommendation: "Create a limited audit Spaces access key or manually validate bucket ACL, bucket policy, CORS, lifecycle and versioning settings in the DigitalOcean control panel.",
				Remediation:    "Confirm the bucket is private unless intentionally public, restrict CORS to approved origins, define lifecycle rules, and enable versioning where recovery is required.",
				Validation:     "Provide Spaces access key/secret for authenticated scan or attach screenshot/export evidence from the Spaces bucket settings.",
				Timeline:       "Medium term / within 1 month",
			})
		}
	}
}

func checkDatabaseFirewallAdvanced(inv model.Inventory, add func(model.Finding)) {
	dbRules := obj(inv.Resources["database_firewall_rules"])

	for dbName, rawRules := range dbRules {
		rules, ok := rawRules.([]interface{})
		if !ok {
			continue
		}

		isProdDB := strings.Contains(strings.ToLower(dbName), "prod")

		for _, raw := range rules {
			rule, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}

			value := str(rule, "value")
			ruleType := str(rule, "type")
			text := strings.ToLower(fmt.Sprintf("%v", rule))

			if value == "0.0.0.0/0" || value == "::/0" {
				add(model.Finding{
					Severity:       "High",
					Category:       "Database Network Access",
					Title:          "Managed database firewall allows access from the internet",
					ResourceType:   "managed_database_firewall",
					ResourceName:   dbName,
					Standard:       "ISO 27001 A.8.20 / NIST PR.AC / CIS Control 4",
					Risk:           "The managed database firewall allows access from any internet source. Database access should normally be limited to application services, private networks, or trusted administrative paths.",
					BusinessImpact: "If database credentials are exposed or guessed, internet-wide network access increases the likelihood of unauthorized database access.",
					Evidence:       fmt.Sprintf("database firewall rule type=%s value=%s", ruleType, value),
					Recommendation: "Restrict database firewall sources to the application, trusted private networks, or approved administrative ranges.",
					Remediation:    "Remove 0.0.0.0/0 and ::/0 from database firewall rules and replace them with least-privilege sources.",
					Validation:     "Re-run the scan and confirm no database firewall rule allows internet-wide access.",
					Timeline:       "Short term / within 1 week",
				})
				continue
			}

			if cidrIsBroad(value) {
				add(model.Finding{
					Severity:       "Medium",
					Category:       "Database Network Access",
					Title:          "Managed database firewall uses a broad CIDR range",
					ResourceType:   "managed_database_firewall",
					ResourceName:   dbName,
					Standard:       "ISO 27001 A.8.20 / NIST PR.AC / CIS Control 4",
					Risk:           "A broad CIDR range may allow more hosts to reach the database than necessary.",
					BusinessImpact: "The database network exposure may be larger than the actual application or administration requirement.",
					Evidence:       fmt.Sprintf("database firewall rule type=%s value=%s", ruleType, value),
					Recommendation: "Narrow database firewall rules to exact application sources or small trusted administrative ranges.",
					Remediation:    "Replace broad CIDRs with specific service sources, VPC/private network paths, or smaller approved ranges.",
					Validation:     "Re-run the scan and confirm broad CIDR ranges are removed or justified.",
					Timeline:       "Medium term / within 1 month",
				})
			}

			if isProdDB && strings.Contains(text, "dev") {
				add(model.Finding{
					Severity:       "Medium",
					Category:       "Environment Separation",
					Title:          "Production database firewall appears to allow development sources",
					ResourceType:   "managed_database_firewall",
					ResourceName:   dbName,
					Standard:       "ISO 27001 A.5.15 / NIST PR.AA / CIS Control 6",
					Risk:           "Production databases should normally be reachable only from production workloads and approved administrative paths.",
					BusinessImpact: "Development workloads may gain a network path to production data if firewall sources are mixed between environments.",
					Evidence:       fmt.Sprintf("production-like database name with firewall rule containing dev context: %v", rule),
					Recommendation: "Review database firewall sources and separate production and development access paths.",
					Remediation:    "Remove development sources from production database firewall rules unless a documented exception exists.",
					Validation:     "Re-run the scan and confirm production database firewall sources do not reference development resources.",
					Timeline:       "Medium term / within 1 month",
				})
			}
		}
	}
}

func checkAppEnvAdvanced(inv model.Inventory, add func(model.Finding)) {
	apps := arr(inv.Resources["apps_detailed"])

	for _, app := range apps {
		appName := str(app, "name")
		spec := obj(app["spec"])
		if appName == "" {
			appName = str(spec, "name")
		}

		for _, comp := range appComponents(spec) {
			compName := appName + "/" + str(comp, "name")
			compName = strings.TrimSuffix(compName, "/")

			for _, env := range arr(comp["envs"]) {
				key := str(env, "key")
				value := str(env, "value")
				keyUpper := strings.ToUpper(key)

				if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
					continue
				}

				if isPublicFrontendSecretMistake(keyUpper, value) {
					add(model.Finding{
						Severity:       "High",
						Category:       "Application Secret Management",
						Title:          "Public frontend environment variable appears to contain a secret",
						ResourceType:   "app_platform_env",
						ResourceName:   compName,
						Standard:       "ISO 27001 A.5.15/A.8.12 / NIST PR.DS / CIS Control 3",
						Risk:           "Frontend variables with NEXT_PUBLIC, PUBLIC or VITE prefixes are normally exposed to browser clients. They should not contain private keys, tokens, passwords or backend secrets.",
						BusinessImpact: "A secret exposed to the frontend can be copied by any user who loads the application.",
						Evidence:       "public env var name contains secret-like token: " + key,
						Recommendation: "Move secrets out of public frontend variables and expose only non-sensitive public configuration.",
						Remediation:    "Rotate the exposed value if it is sensitive and replace it with a backend-only secret or server-side proxy.",
						Validation:     "Re-run the scan and confirm public frontend variables do not contain secret-like names or plaintext sensitive values.",
						Timeline:       "Immediate / today",
					})
					continue
				}

				if isHighRiskEnvName(keyUpper) && !isEncryptedDOEnvValue(value) {
					add(model.Finding{
						Severity:       "High",
						Category:       "Application Secret Management",
						Title:          "High-risk application environment variable is stored as plaintext",
						ResourceType:   "app_platform_env",
						ResourceName:   compName,
						Standard:       "ISO 27001 A.5.15/A.8.12 / NIST PR.DS / CIS Control 3",
						Risk:           "High-risk environment variables should be stored as encrypted platform secrets rather than plaintext values in application configuration.",
						BusinessImpact: "Plaintext secrets in exported configuration increase the chance of credential exposure during audit, troubleshooting or repository handling.",
						Evidence:       "plaintext high-risk env var detected: " + key,
						Recommendation: "Store sensitive environment variables as encrypted App Platform secrets.",
						Remediation:    "Move this value to an encrypted secret, rotate it if it was exposed, and restrict access to raw evidence exports.",
						Validation:     "Re-run the scan and confirm the value is encrypted or no longer exported as plaintext.",
						Timeline:       "Short term / within 1 week",
					})
				}

				if isProd(appName, nil) && envValueLooksDev(value) {
					add(model.Finding{
						Severity:       "Medium",
						Category:       "Environment Separation",
						Title:          "Production application environment references development configuration",
						ResourceType:   "app_platform_env",
						ResourceName:   compName,
						Standard:       "ISO 27001 A.5.15 / NIST PR.AA / CIS Control 6",
						Risk:           "Production applications should not normally reference development hosts, localhost URLs or development services.",
						BusinessImpact: "Environment mixing can cause production outages, data leakage or accidental dependency on non-production services.",
						Evidence:       "production app env var appears to reference development context: " + key,
						Recommendation: "Review production environment variables and remove development references unless explicitly justified.",
						Remediation:    "Replace development URLs or service references with production-approved values.",
						Validation:     "Re-run the scan and confirm production env vars no longer reference development context.",
						Timeline:       "Medium term / within 1 month",
					})
				}
			}
		}
	}
}

func cidrIsBroad(value string) bool {
	ip, network, err := net.ParseCIDR(strings.TrimSpace(value))
	if err != nil || ip == nil || network == nil {
		return false
	}

	ones, bits := network.Mask.Size()
	if bits == 32 && ones <= 16 {
		return true
	}
	if bits == 128 && ones <= 64 {
		return true
	}

	return false
}

func isEncryptedDOEnvValue(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), "EV[")
}

func isHighRiskEnvName(key string) bool {
	key = strings.ToUpper(key)

	allowedPublic := []string{
		"NEXT_PUBLIC_SENTRY_DSN",
		"NEXT_PUBLIC_SUPABASE_URL",
		"NEXT_PUBLIC_SUPABASE_ANON_KEY",
	}

	for _, allowed := range allowedPublic {
		if key == allowed {
			return false
		}
	}

	for _, token := range []string{
		"PASSWORD",
		"PASSWD",
		"SECRET",
		"PRIVATE_KEY",
		"API_KEY",
		"ACCESS_TOKEN",
		"REFRESH_TOKEN",
		"AUTH_TOKEN",
		"JWT_SECRET",
		"DATABASE_URL",
		"POSTGRES_URL",
		"REDIS_URL",
		"STRIPE_SECRET",
		"SENDGRID_API_KEY",
		"TWILIO_AUTH_TOKEN",
		"OPENAI_API_KEY",
		"ANTHROPIC_API_KEY",
		"DIGITALOCEAN_TOKEN",
		"DO_TOKEN",
		"SPACES_SECRET",
		"AWS_SECRET_ACCESS_KEY",
	} {
		if strings.Contains(key, token) {
			return true
		}
	}

	return false
}

func isPublicFrontendSecretMistake(key string, value string) bool {
	key = strings.ToUpper(key)

	if !strings.HasPrefix(key, "NEXT_PUBLIC_") &&
		!strings.HasPrefix(key, "PUBLIC_") &&
		!strings.HasPrefix(key, "VITE_") {
		return false
	}

	if isEncryptedDOEnvValue(value) {
		return false
	}

	return isHighRiskEnvName(key)
}

func envValueLooksDev(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))

	return strings.Contains(v, "localhost") ||
		strings.Contains(v, "127.0.0.1") ||
		strings.Contains(v, "dev.") ||
		strings.Contains(v, "-dev") ||
		strings.Contains(v, "_dev") ||
		strings.Contains(v, "development")
}
