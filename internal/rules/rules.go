package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"infra-audit/internal/model"
)

func Evaluate(inv model.Inventory) []model.Finding {
	var findings []model.Finding
	n := 1

	add := func(f model.Finding) {
		f.ID = fmt.Sprintf("F-%03d", n)
		if f.Status == "" {
			f.Status = "Open"
		}
		if f.Priority == "" {
			f.Priority = defaultPriority(f.Severity)
		}
		if f.Timeline == "" {
			f.Timeline = defaultTimeline(f.Severity)
		}
		findings = append(findings, f)
		n++
	}

	checkSecrets(inv, add)
	checkAccount(inv, add)
	checkDroplets(inv, add)
	checkFirewalls(inv, add)
	checkDatabases(inv, add)
	checkKubernetes(inv, add)
	checkLoadBalancers(inv, add)
	checkReservedIPs(inv, add)
	checkApps(inv, add)
	checkAppPlatformDeep(inv, add)
	checkSpacesLite(inv, add)
	checkDatabaseFirewallAdvanced(inv, add)
	checkAppEnvAdvanced(inv, add)
	checkContainerRegistry(inv, add)

	sort.SliceStable(findings, func(i, j int) bool {
		if sevRank(findings[i].Severity) == sevRank(findings[j].Severity) {
			return findings[i].ID < findings[j].ID
		}
		return sevRank(findings[i].Severity) > sevRank(findings[j].Severity)
	})

	return findings
}

func checkAppPlatformDeep(inv model.Inventory, add func(model.Finding)) {
	apps := arr(inv.Resources["apps_detailed"])
	if len(apps) == 0 {
		return
	}

	corsReported := false
	regionReported := false
	deployAlertReported := map[string]bool{}
	sentryReported := false
	supabaseReported := false

	backendRegions := map[string]bool{}
	frontendRegions := map[string]bool{}

	for _, app := range apps {
		appName := str(app, "name")
		spec := obj(app["spec"])
		if appName == "" {
			appName = str(spec, "name")
		}

		region := appRegion(app, spec)
		appText := fmt.Sprintf("%v", app)

		lowerName := strings.ToLower(appName)
		if isProd(appName, nil) {
			if strings.Contains(lowerName, "backend") || strings.Contains(lowerName, "api") || strings.Contains(lowerName, "server") {
				if region != "" {
					backendRegions[region] = true
				}
			}
			if strings.Contains(lowerName, "frontend") || strings.Contains(lowerName, "tenant") || strings.Contains(lowerName, "fe") || strings.Contains(lowerName, "web") {
				if region != "" {
					frontendRegions[region] = true
				}
			}
		}

		if !corsReported && strings.Contains(strings.ToLower(appText), "localhost:3000") {
			corsReported = true
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Application Security Configuration",
				Title:          "Development CORS origin is present in cloud application configuration",
				ResourceType:   "app_platform",
				ResourceName:   appName,
				Standard:       "ISO 27001 A.8.20 / NIST PR.AC / CIS Control 4",
				Risk:           "A localhost development origin was found in cloud application configuration. This is not automatically a security vulnerability, but it is a sign of configuration drift and should be cleaned up so production and development access rules remain clear.",
				BusinessImpact: "Unclear CORS configuration can break frontend/API traffic, complicate authentication troubleshooting, or lead to rushed production changes during releases.",
				Evidence:       "App Platform specification contains localhost:3000.",
				Recommendation: "Normalize CORS configuration across environments and drive allowed origins from explicit environment-specific variables.",
				Remediation:    "Replace hard-coded development origins with explicit per-environment configuration and document the expected production origins.",
				Validation:     "Re-run the scan and confirm localhost origins are absent from production App Platform specs.",
			})
		}

		for _, comp := range appComponents(spec) {
			compName := appName + "/" + str(comp, "name")
			if compName == appName+"/" {
				compName = appName
			}

			if appHasDisabledDeploymentAlert(comp) && !deployAlertReported[compName] {
				deployAlertReported[compName] = true
				add(model.Finding{
					Severity:       "Medium",
					Category:       "Monitoring and Alerting",
					Title:          "Deployment failure alert is disabled",
					ResourceType:   "app_platform_component",
					ResourceName:   compName,
					Standard:       "ISO 27001 A.8.16 / NIST DE.CM",
					Risk:           "Failed deployments may not notify the team automatically.",
					BusinessImpact: "A broken production release may remain undetected until users report the issue.",
					Evidence:       "DEPLOYMENT_FAILED alert has disabled=true.",
					Recommendation: "Enable deployment failure alerts for production and business-critical applications.",
					Remediation:    "Set DEPLOYMENT_FAILED alert disabled=false and verify notification delivery.",
					Validation:     "Trigger or simulate a failed deployment notification test.",
				})
			}

			for _, env := range arr(comp["envs"]) {
				key := strings.ToUpper(str(env, "key"))
				val := str(env, "value")

				if key == "NEXT_PUBLIC_SENTRY_DSN" && strings.TrimSpace(val) != "" && !strings.HasPrefix(val, "EV[") && !sentryReported {
					sentryReported = true
					add(model.Finding{
						Severity:       "Low",
						Category:       "Monitoring and Telemetry",
						Title:          "Sentry DSN is exposed as a public frontend variable",
						ResourceType:   "app_platform_env",
						ResourceName:   compName,
						Standard:       "ISO 27001 A.8.15 / NIST DE.CM",
						Risk:           "Public Sentry DSNs are expected in frontend applications, but unrestricted ingest can allow fabricated events and noise in monitoring.",
						BusinessImpact: "Error monitoring may be polluted with fake events, causing alert fatigue or hiding real incidents.",
						Evidence:       "NEXT_PUBLIC_SENTRY_DSN is present as a plaintext public variable.",
						Recommendation: "Configure allowed ingest origins in Sentry and monitor event volume anomalies.",
						Remediation:    "Restrict Sentry project allowed domains/origins to approved production domains.",
						Validation:     "Confirm Sentry accepts events only from approved application origins.",
					})
				}

				if strings.Contains(key, "SUPABASE_SERVICE_ROLE") && !supabaseReported {
					supabaseReported = true
					add(model.Finding{
						Severity:       "Medium",
						Category:       "Identity and Data Access",
						Title:          "Supabase service-role key separation requires manual validation",
						ResourceType:   "app_platform_env",
						ResourceName:   compName,
						Standard:       "ISO 27001 A.5.15 / NIST PR.AA / CIS Control 6",
						Risk:           "Supabase service-role keys can bypass row-level security, so production and non-production keys should be kept separate and tightly controlled.",
						BusinessImpact: "If the same service-role key is reused across environments, a development or staging workload could gain unintended access to production data.",
						Evidence:       "A SUPABASE_SERVICE_ROLE_KEY-like variable exists in App Platform configuration. The value is encrypted in the API export, so key uniqueness cannot be verified automatically.",
						Recommendation: "Document which Supabase project belongs to each environment and confirm that service-role keys are unique per environment.",
						Remediation:    "If any shared keys are found during manual validation, rotate them and maintain separate production and development Supabase projects.",
						Validation:     "Manually verify Supabase project IDs and service-role keys outside the encrypted App Platform export.",
					})
				}
			}
		}
	}

	if !regionReported && len(backendRegions) > 0 && len(frontendRegions) > 0 && !mapsOverlap(backendRegions, frontendRegions) {
		regionReported = true
		add(model.Finding{
			Severity:       "Medium",
			Category:       "Architecture and Resilience",
			Title:          "Backend and frontend applications are deployed in different regions",
			ResourceType:   "app_platform",
			ResourceName:   "App Platform regional placement",
			Standard:       "ISO 27001 A.5.30 / NIST PR.PT",
			Risk:           "Cross-region application placement increases latency and can prevent private-network communication patterns.",
			BusinessImpact: "Users may experience slower API calls, and the architecture may become harder to operate during incidents.",
			Evidence:       "Production-like backend regions and frontend regions differ in App Platform configuration.",
			Recommendation: "Evaluate consolidating frontend and backend applications into the same primary region unless a documented reason exists.",
			Remediation:    "Choose the target region and migrate legacy components during an approved maintenance window.",
			Validation:     "Re-run the scan and confirm production frontend/backend resources share the expected region.",
		})
	}
}

func appRegion(app map[string]interface{}, spec map[string]interface{}) string {
	for _, key := range []string{"region", "region_slug"} {
		if v := str(app, key); v != "" {
			return v
		}
		if v := str(spec, key); v != "" {
			return v
		}
	}
	return ""
}

func appComponents(spec map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	for _, key := range []string{"services", "static_sites", "workers", "jobs"} {
		out = append(out, arr(spec[key])...)
	}
	return out
}

func appHasDisabledDeploymentAlert(comp map[string]interface{}) bool {
	for _, alert := range arr(comp["alerts"]) {
		rule := strings.ToUpper(str(alert, "rule"))
		if strings.Contains(rule, "DEPLOYMENT_FAILED") && boolVal(alert["disabled"]) {
			return true
		}
	}
	return false
}

func mapsOverlap(a map[string]bool, b map[string]bool) bool {
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}

func PositiveFindings(inv model.Inventory) []model.PositiveFinding {
	var out []model.PositiveFinding

	appText := fmt.Sprintf("%v", inv.Resources["apps_detailed"])

	if len(arr(inv.Resources["apps"])) > 0 {
		out = append(out, model.PositiveFinding{
			Area:     "Managed application platform",
			Status:   "Observed",
			Evidence: fmt.Sprintf("%d App Platform app(s) discovered in scope.", len(arr(inv.Resources["apps"]))),
		})
	}

	if strings.Contains(appText, "EV[") {
		out = append(out, model.PositiveFinding{
			Area:     "App Platform encrypted secret values",
			Status:   "Observed",
			Evidence: "At least one App Platform specification uses encrypted EV[...] secret values.",
		})
	}

	if strings.Contains(strings.ToLower(appText), "prod") && strings.Contains(strings.ToLower(appText), "dev") {
		out = append(out, model.PositiveFinding{
			Area:     "Production and development separation",
			Status:   "Observed",
			Evidence: "Production and development application naming/environments were both observed in App Platform evidence.",
		})
	}

	if strings.Contains(strings.ToUpper(appText), "SENTRY") {
		out = append(out, model.PositiveFinding{
			Area:     "Sentry monitoring",
			Status:   "Observed",
			Evidence: "Sentry-related configuration was detected in application environment settings.",
		})
	}

	if strings.Contains(strings.ToLower(appText), "health") {
		out = append(out, model.PositiveFinding{
			Area:     "Health check configuration",
			Status:   "Observed",
			Evidence: "Health-check related configuration was detected in App Platform specifications.",
		})
	}

	if strings.Contains(strings.ToLower(appText), "autoscaling") {
		out = append(out, model.PositiveFinding{
			Area:     "Application autoscaling",
			Status:   "Partially implemented",
			Evidence: "Autoscaling configuration was detected for at least one App Platform component.",
		})
	}

	if strings.Contains(strings.ToUpper(appText), "SUPABASE") {
		out = append(out, model.PositiveFinding{
			Area:     "Supabase integration",
			Status:   "Observed",
			Evidence: "Supabase-related configuration was detected in application environment settings.",
		})
	}

	for _, db := range arr(inv.Resources["databases"]) {
		if str(db, "private_network_uuid") != "" {
			out = append(out, model.PositiveFinding{
				Area:     "Database private networking",
				Status:   "Observed / partially implemented",
				Evidence: "At least one managed database has private_network_uuid configured.",
			})
			break
		}
	}

	if len(arr(inv.Resources["vpcs"])) > 0 {
		out = append(out, model.PositiveFinding{
			Area:     "VPC/private networking foundation",
			Status:   "Observed",
			Evidence: fmt.Sprintf("%d VPC network(s) discovered in scope.", len(arr(inv.Resources["vpcs"]))),
		})
	}

	if strings.Contains(strings.ToUpper(appText), "CPU") || strings.Contains(strings.ToUpper(appText), "MEMORY") {
		out = append(out, model.PositiveFinding{
			Area:     "Resource-level operational monitoring",
			Status:   "Observed",
			Evidence: "CPU or memory alert-related configuration was detected in App Platform evidence.",
		})
	}

	return out
}

func SummaryText(r model.Report) string {
	c := CountBySeverity(r.Findings)
	total := len(r.Findings)

	if total == 0 {
		return fmt.Sprintf(
			"The review did not identify automated findings from the %s evidence available to the scanner. This does not mean the environment is risk-free; it means no issues were detected by the current automated checks. Manual validation is still recommended for identity access, application authorization, data classification, incident response, and third-party services.",
			r.Meta.Provider,
		)
	}

	return fmt.Sprintf(
		"The review identified %d security, resilience, and configuration findings across the %s environment: %d Critical, %d High, %d Medium, and %d Low/Info. Priority actions should focus on the following areas: %s. The environment has a solid cloud foundation, including managed application services and managed databases, but several configuration gaps should be addressed to reduce exposure and improve operational resilience. Findings are based on exported cloud configuration evidence collected during this assessment and should be validated with the client before remediation begins.",
		total,
		r.Meta.Client,
		c["Critical"],
		c["High"],
		c["Medium"],
		c["Low"]+c["Info"],
		summaryFocusAreas(r.Findings),
	)
}

func summaryFocusAreas(findings []model.Finding) string {
	areas := []string{}
	added := map[string]bool{}

	add := func(area string) {
		if area == "" || added[area] {
			return
		}
		added[area] = true
		areas = append(areas, area)
	}

	for _, f := range findings {
		text := strings.ToLower(f.Title + " " + f.Category + " " + f.Risk)

		switch {
		case strings.Contains(text, "credential") || strings.Contains(text, "secret"):
			add("protect exported evidence and secrets")
		case strings.Contains(text, "ssh") || strings.Contains(text, "internet") || strings.Contains(text, "firewall"):
			add("reduce internet exposure and enforce firewall policy")
		case strings.Contains(text, "database") || strings.Contains(text, "standby") || strings.Contains(text, "maintenance"):
			add("improve database resilience and maintenance")
		case strings.Contains(text, "backup") || strings.Contains(text, "snapshot"):
			add("improve backup and recovery coverage")
		case strings.Contains(text, "cors") || strings.Contains(text, "supabase"):
			add("validate application configuration and environment separation")
		case strings.Contains(text, "reserved ip") || strings.Contains(text, "dns"):
			add("clean up stable endpoint and DNS configuration")
		case strings.Contains(text, "autoscaling") || strings.Contains(text, "alert"):
			add("improve availability and operational monitoring")
		}
	}

	if len(areas) == 0 {
		return "configuration hardening and control validation"
	}

	if len(areas) > 4 {
		areas = areas[:4]
	}

	return strings.Join(areas, ", ")
}

func Limitations(inv model.Inventory) []string {
	out := []string{
		"This is a point-in-time automated configuration review based on resources visible to the supplied API token.",
		"The assessment does not include penetration testing, authenticated host vulnerability scanning, source-code review, malware analysis or manual SaaS/IdP review.",
		"Some controls require manual validation: business ownership, data classification, backup restore testing, incident response procedures, vendor configuration and application authorization logic.",
		"Raw evidence exports must be treated as confidential because cloud API data may include sensitive configuration values.",
	}

	if len(inv.Errors) > 0 {
		out = append(out, "Some API collections returned errors. Review collection warnings before treating missing resources as proof that they do not exist.")
	}

	return out
}

func CountBySeverity(findings []model.Finding) map[string]int {
	m := map[string]int{"Critical": 0, "High": 0, "Medium": 0, "Low": 0, "Info": 0}
	for _, f := range findings {
		if _, ok := m[f.Severity]; !ok {
			m[f.Severity] = 0
		}
		m[f.Severity]++
	}
	return m
}

func checkSecrets(inv model.Inventory, add func(model.Finding)) {
	paths := findSecretPaths(inv.Resources)
	if len(paths) == 0 {
		return
	}

	shown := paths
	if len(shown) > 12 {
		shown = shown[:12]
	}

	add(model.Finding{
		Severity:     "Critical",
		Category:     "Secrets Management",
		Title:        "Raw cloud evidence requires secure handling because it contains sensitive values",
		ResourceType: "evidence_export",
		ResourceName: "cloud API export JSON",
		Standard:     "ISO 27001 A.5.15/A.8.12 / NIST PR.DS / CIS Control 3",
		ControlMapping: []string{
			"ISO 27001: Access control and information protection",
			"NIST CSF: Protect / Data Security",
			"CIS Controls: Data Protection",
		},
		Risk:           "The DigitalOcean API export includes database passwords and connection details. This can be normal for inventory collection, but the exported file becomes sensitive audit evidence. The main risk is improper handling of the raw export, such as committing it to a repository, uploading it to shared storage, or sending it without encryption.",
		BusinessImpact: "If the raw export leaves the approved audit workspace or is stored without proper access controls, it could expose credentials that may allow unauthorized access.",
		Evidence:       "Credential-like fields found at: " + strings.Join(shown, "; ") + " — values are redacted in this report.",
		Recommendation: "Treat raw evidence files as confidential audit material. Confirm where the export was stored, who had access to it, and whether it was shared outside the approved audit workspace. Rotate credentials if exposure is confirmed.",
		Remediation:    "Encrypt or securely delete raw exports after use, restrict access to evidence files, and keep runtime secrets in a secrets manager. Rotate credentials only if exposure outside the approved audit workspace is confirmed.",
		Validation:     "If credentials are rotated, confirm old credentials no longer authenticate. Re-run the scan to verify report output redacts credential values and evidence storage is controlled.",
	})
}

func checkAccount(inv model.Inventory, add func(model.Finding)) {
	account := obj(inv.Resources["account"])
	if b, ok := account["email_verified"].(bool); ok && !b {
		add(model.Finding{
			Severity:       "High",
			Category:       "Identity and Account Security",
			Title:          "DigitalOcean account email is not verified",
			ResourceType:   "account",
			ResourceName:   str(account, "email"),
			Standard:       "ISO 27001 A.5.16 / NIST PR.AA",
			Risk:           "Account recovery and ownership verification may be weakened.",
			BusinessImpact: "The organization may have reduced ability to recover or prove ownership during an account-security incident.",
			Evidence:       "account.email_verified=false",
			Recommendation: "Verify account email and review owner/admin access.",
			Remediation:    "Complete email verification and document cloud account owners.",
			Validation:     "Re-run scan and confirm email_verified=true.",
		})
	}
}

func checkDroplets(inv model.Inventory, add func(model.Finding)) {
	droplets := arr(inv.Resources["droplets"])
	firewalls := arr(inv.Resources["firewalls"])
	protectedDroplets, protectedTags := firewallCoverage(firewalls)

	for _, d := range droplets {
		id := idString(d["id"])
		name := str(d, "name")
		publicIP := dropletPublicIPv4(d)

		if publicIP != "" && !isDropletProtected(d, protectedDroplets, protectedTags) {
			add(model.Finding{
				Severity:           "High",
				Category:           "Network Security",
				Title:              "Public Droplet is not protected by a DigitalOcean Cloud Firewall",
				ResourceType:       "droplet",
				ResourceName:       name,
				ResourceID:         id,
				AffectedComponents: []string{name, publicIP},
				Standard:           "CIS Controls 4, 12 / NIST PR.PT / ISO 27001 A.8.20",
				Risk:               "The Droplet has a public IP address, but no matching DigitalOcean Cloud Firewall attachment was found. This increases exposure to internet scanning and brute-force attempts against any listening services.",
				BusinessImpact:     "If a vulnerable or weakly protected service is reachable, the host could become an entry point for downtime, data exposure, credential theft, or lateral movement.",
				Evidence:           "public_ipv4=" + publicIP + "; no matching firewall by droplet_id or tag",
				Recommendation:     "Place the Droplet behind an effective Cloud Firewall policy.",
				Remediation:        "Attach an existing firewall or apply a protected tag. Restrict SSH to VPN, bastion, or trusted office IPs, and allow only required application ports.",
				Validation:         "Confirm the firewall object lists this Droplet ID or a tag used by this Droplet.",
			})
		}

		if !hasString(sliceStrings(d["features"]), "backups") {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Backup and Recovery",
				Title:          "Droplet backup coverage is not enabled",
				ResourceType:   "droplet",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.5.30 / CIS Control 11 / NIST PR.IP",
				Risk:           "Recovery options may be limited if the host is deleted, compromised, misconfigured, or affected by operator error.",
				BusinessImpact: "The client may experience extended downtime or irreversible configuration loss.",
				Evidence:       "features does not include backups",
				Recommendation: "Enable automated backups or implement a documented snapshot schedule with periodic restore testing.",
				Remediation:    "Enable weekly backups or schedule snapshots after hardening milestones and before major changes.",
				Validation:     "Confirm the Droplet feature list includes backups or that a recent snapshot exists.",
			})
		}

		if isRetiredImage(d) {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Patch and Vulnerability Management",
				Title:          "Droplet is running a retired or deprecated image",
				ResourceType:   "droplet",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.8.8 / CIS Control 7 / NIST PR.IP",
				Risk:           "Retired or non-LTS images may stop receiving security patches.",
				BusinessImpact: "Known vulnerabilities in the OS may remain unpatched.",
				Evidence:       "image.status indicates retired/deprecated or image metadata indicates retired status",
				Recommendation: "Migrate to a supported LTS image and maintain a patch-management schedule.",
				Remediation:    "Snapshot the Droplet, rebuild/migrate to a supported LTS release, validate services, then decommission the old host.",
				Validation:     "Re-run scan and confirm image status is supported.",
			})
		}

		if len(sliceStrings(d["tags"])) == 0 {
			add(model.Finding{
				Severity:       "Low",
				Category:       "Asset Management",
				Title:          "Droplet ownership metadata is missing",
				ResourceType:   "droplet",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "CIS Control 1 / NIST ID.AM / ISO 27001 A.5.9",
				Risk:           "Without tags, it is harder to identify the owner, environment, and business criticality of the asset during operations or incident response.",
				BusinessImpact: "Operational handoff, cost attribution and incident triage may be slower.",
				Evidence:       "tags=[]",
				Recommendation: "Apply standard tags such as client, environment, owner, data classification, and criticality.",
				Remediation:    "Define and apply a minimum tag taxonomy.",
				Validation:     "Re-run scan and confirm required tags exist.",
			})
		}
	}
}

func checkFirewalls(inv model.Inventory, add func(model.Finding)) {
	for _, fw := range arr(inv.Resources["firewalls"]) {
		name := str(fw, "name")
		id := str(fw, "id")

		for _, rule := range arr(fw["inbound_rules"]) {
			proto := str(rule, "protocol")
			ports := str(rule, "ports")

			if isWorldOpen(rule) {
				sev := severityForOpenPort(proto, ports)
				if sev == "" {
					continue
				}

				add(model.Finding{
					Severity:       sev,
					Category:       "Network Security",
					Title:          "Firewall allows sensitive inbound access from the internet",
					ResourceType:   "firewall",
					ResourceName:   name,
					ResourceID:     id,
					Standard:       "CIS Controls 4, 12 / NIST PR.AC / ISO 27001 A.8.20",
					Risk:           "The firewall policy allows sensitive access from any internet source. For management ports such as SSH, this should normally be restricted to VPN, bastion, or trusted administrative IP ranges.",
					BusinessImpact: "If the rule is attached to an active resource, unauthorized users can repeatedly attempt access from anywhere on the internet.",
					Evidence:       fmt.Sprintf("protocol=%s ports=%s sources include 0.0.0.0/0 or ::/0", proto, ports),
					Recommendation: "Restrict inbound source CIDRs to VPN, bastion host, or trusted office IP ranges.",
					Remediation:    "Replace world-open source rules with trusted CIDRs. Use a bastion, VPN, or identity-aware access path for administration.",
					Validation:     "Confirm sensitive ports are no longer reachable from arbitrary internet sources.",
				})
			}
		}

		if len(anySlice(fw["droplet_ids"])) == 0 && len(sliceStrings(fw["tags"])) == 0 {
			add(model.Finding{
				Severity:       "High",
				Category:       "Network Security",
				Title:          "Cloud firewall is defined but not attached to any Droplet or tag",
				ResourceType:   "firewall",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "CIS Control 4 / NIST PR.PT / ISO 27001 A.8.20",
				Risk:           "The firewall object exists but is not attached to any Droplet or tag. This means the policy is not enforcing traffic controls on any resource.",
				BusinessImpact: "This can create a false sense of protection if the team expects this firewall to cover production or development hosts.",
				Evidence:       "droplet_ids=[] and tags=[]",
				Recommendation: "Attach the firewall to required Droplets/tags or remove unused policy after review.",
				Remediation:    "Attach the firewall to the intended Droplet IDs or to a consistent tag used by the target workloads.",
				Validation:     "Re-run scan and confirm droplet_ids or tags are populated.",
			})
		}
	}
}

func checkDatabases(inv model.Inventory, add func(model.Finding)) {
	dbFirewall := obj(inv.Resources["database_firewall_rules"])

	for _, db := range arr(inv.Resources["databases"]) {
		name := str(db, "name")
		id := str(db, "id")
		prod := isProd(name, sliceStrings(db["tags"]))

		if str(db, "private_network_uuid") == "" {
			add(model.Finding{
				Severity:       "High",
				Category:       "Database Network Security",
				Title:          "Managed database is not attached to a private VPC network",
				ResourceType:   "database",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.8.20 / NIST PR.PT / CIS Control 4",
				Risk:           "Database traffic may rely on public connectivity instead of private network isolation.",
				BusinessImpact: "Database compromise risk increases if access depends only on passwords and public endpoint controls.",
				Evidence:       "private_network_uuid is empty",
				Recommendation: "Place managed databases in a VPC/private network and restrict access using trusted sources.",
				Remediation:    "Attach the database to the expected VPC and update applications to use private endpoints.",
				Validation:     "Confirm private_network_uuid is populated and applications use private connectivity.",
			})
		}

		if prod && numeric(db["num_nodes"]) < 2 {
			add(model.Finding{
				Severity:       "High",
				Category:       "Availability and Resilience",
				Title:          "Production database is running without a standby node",
				ResourceType:   "database",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.5.30 / NIST PR.PT / CIS Control 11",
				Risk:           "The production database is running with a single node, which creates a single point of failure.",
				BusinessImpact: "A node failure or maintenance event could cause production downtime until the database service is restored.",
				Evidence:       fmt.Sprintf("production-like name/tags detected; num_nodes=%v", db["num_nodes"]),
				Recommendation: "Add a standby node or replica for production databases and verify that backups can be restored.",
				Remediation:    "Increase num_nodes to at least 2 for production database clusters.",
				Validation:     "Re-run scan and confirm num_nodes>=2.",
			})
		}

		if maint := obj(db["maintenance_window"]); boolVal(maint["pending"]) {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Patch and Vulnerability Management",
				Title:          "Managed database has pending maintenance",
				ResourceType:   "database",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.8.8 / CIS Control 7 / NIST PR.IP",
				Risk:           "Pending database maintenance may include security, reliability, or platform updates and should be scheduled within an approved maintenance window.",
				BusinessImpact: "Delayed maintenance can prolong exposure to known vulnerabilities or reliability fixes.",
				Evidence:       "maintenance_window.pending=true",
				Recommendation: "Apply pending maintenance during the next approved maintenance window after confirming backups and rollback steps.",
				Remediation:    "Verify backups, apply maintenance and monitor application health.",
				Validation:     "Re-run scan and confirm maintenance_window.pending=false.",
			})
		}

		if rules, ok := dbFirewall[name]; ok && len(anySlice(rules)) == 0 {
			add(model.Finding{
				Severity:       "High",
				Category:       "Database Access Control",
				Title:          "Managed database firewall has no allow-list rules configured",
				ResourceType:   "database",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.5.15 / NIST PR.AC / CIS Control 6",
				Risk:           "The database may rely on broad/default exposure and credential-only protection.",
				BusinessImpact: "Unauthorized network paths to the database increase brute-force and credential replay risk.",
				Evidence:       "database_firewall_rules for this cluster is empty",
				Recommendation: "Restrict database access to application private networks or trusted service resources.",
				Remediation:    "Configure database trusted sources for required apps, Droplets, VPCs or CIDR ranges.",
				Validation:     "Re-run scan and confirm firewall rules are populated.",
			})
		}

		if len(sliceStrings(db["tags"])) == 0 {
			add(model.Finding{
				Severity:       "Low",
				Category:       "Asset Management",
				Title:          "Managed database ownership metadata is missing",
				ResourceType:   "database",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "CIS Control 1 / NIST ID.AM / ISO 27001 A.5.9",
				Risk:           "Ownership, environment, and criticality are not visible in inventory metadata.",
				BusinessImpact: "Data classification and incident ownership may be unclear.",
				Evidence:       "tags=[]",
				Recommendation: "Apply standard tags and document data classification.",
				Remediation:    "Add environment, owner, data classification, and criticality tags.",
				Validation:     "Re-run scan and confirm tags exist.",
			})
		}
	}
}

func checkKubernetes(inv model.Inventory, add func(model.Finding)) {
	for _, k := range arr(inv.Resources["kubernetes_clusters"]) {
		name := str(k, "name")
		id := str(k, "id")

		if !boolVal(k["auto_upgrade"]) {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Kubernetes Security",
				Title:          "Kubernetes auto-upgrade is disabled",
				ResourceType:   "kubernetes_cluster",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "CIS Kubernetes / NIST PR.IP / ISO 27001 A.8.8",
				Risk:           "Cluster version may lag behind security patches and supported baselines.",
				BusinessImpact: "Known Kubernetes/node vulnerabilities may remain unpatched longer than necessary.",
				Evidence:       "auto_upgrade=false",
				Recommendation: "Enable auto-upgrade where operationally acceptable or define a monthly upgrade process.",
				Remediation:    "Enable auto-upgrade or document a recurring patch cycle.",
				Validation:     "Confirm auto_upgrade=true or an approved exception exists.",
			})
		}

		if k["control_plane_firewall"] == nil {
			add(model.Finding{
				Severity:       "High",
				Category:       "Kubernetes Security",
				Title:          "Kubernetes control plane firewall is not configured",
				ResourceType:   "kubernetes_cluster",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "CIS Kubernetes / NIST PR.AC / ISO 27001 A.8.20",
				Risk:           "The Kubernetes API endpoint may be reachable from broad networks.",
				BusinessImpact: "An exposed API server increases brute-force, token replay and exploitation risk.",
				Evidence:       "control_plane_firewall=null",
				Recommendation: "Restrict Kubernetes API access to trusted admin networks/VPN/bastion.",
				Remediation:    "Configure allowed source CIDRs for the control plane firewall.",
				Validation:     "Confirm control_plane_firewall contains approved source ranges.",
			})
		}
	}
}

func checkLoadBalancers(inv model.Inventory, add func(model.Finding)) {
	for _, lb := range arr(inv.Resources["load_balancers"]) {
		name := str(lb, "name")
		id := str(lb, "id")

		if hasHTTPForwarding(lb) && !boolVal(lb["redirect_http_to_https"]) {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Transport Security",
				Title:          "Load balancer does not redirect HTTP to HTTPS",
				ResourceType:   "load_balancer",
				ResourceName:   name,
				ResourceID:     id,
				Standard:       "ISO 27001 A.8.24 / NIST PR.DS",
				Risk:           "Users may access the application over cleartext HTTP.",
				BusinessImpact: "Reduced confidentiality and integrity for user traffic.",
				Evidence:       "forwarding_rules include HTTP and redirect_http_to_https=false",
				Recommendation: "Enable HTTP to HTTPS redirect and verify TLS certificates.",
				Remediation:    "Enable redirect_http_to_https or remove the HTTP listener if not required.",
				Validation:     "HTTP requests return 301/308 redirect to HTTPS.",
			})
		}
	}
}

func checkReservedIPs(inv model.Inventory, add func(model.Finding)) {
	for _, ip := range arr(inv.Resources["reserved_ips"]) {
		addr := str(ip, "ip")
		if ip["droplet"] == nil {
			add(model.Finding{
				Severity:       "Medium",
				Category:       "Availability and DNS Resilience",
				Title:          "Reserved IP is allocated but not attached",
				ResourceType:   "reserved_ip",
				ResourceName:   addr,
				ResourceID:     addr,
				Standard:       "ISO 27001 A.5.30 / NIST PR.PT",
				Risk:           "A reserved IP is allocated but not attached to a host. This may be unused cost, an incomplete failover setup, or a DNS cutover that was not finished.",
				BusinessImpact: "If DNS is expected to use this reserved IP, leaving it unattached can make failover or host replacement harder during an incident.",
				Evidence:       "reserved_ip.droplet=null",
				Recommendation: "Attach the reserved IP to the intended host, or remove it if it is no longer needed.",
				Remediation:    "Assign the reserved IP to the intended host and update DNS A records if applicable.",
				Validation:     "Confirm reserved_ip.droplet is populated.",
			})
		}
	}
}

func checkApps(inv model.Inventory, add func(model.Finding)) {
	for _, app := range arr(inv.Resources["apps_detailed"]) {
		id := str(app, "id")
		name := str(app, "name")
		spec := obj(app["spec"])
		if name == "" {
			name = str(spec, "name")
		}

		for _, svc := range arr(spec["services"]) {
			svcName := str(svc, "name")
			fullName := strings.Trim(name+"/"+svcName, "/")
			prod := isProd(fullName, sliceStrings(svc["tags"]))

			for _, alert := range arr(svc["alerts"]) {
				rule := strings.ToUpper(str(alert, "rule"))
				if prod && strings.Contains(rule, "DEPLOYMENT_FAILED") && boolVal(alert["disabled"]) {
					add(model.Finding{
						Severity:       "Medium",
						Category:       "Monitoring and Alerting",
						Title:          "Production deployment failure alert is disabled",
						ResourceType:   "app_platform_service",
						ResourceName:   fullName,
						ResourceID:     id,
						Standard:       "ISO 27001 A.8.16 / NIST DE.CM",
						Risk:           "Failed deployments may not notify the team automatically.",
						BusinessImpact: "Broken production releases may remain undetected until users report issues.",
						Evidence:       "DEPLOYMENT_FAILED alert has disabled=true",
						Recommendation: "Enable deployment failure alerts for production services.",
						Remediation:    "Set disabled=false and verify notification channels.",
						Validation:     "Trigger or simulate notification test.",
					})
				}
			}

			if prod && numeric(svc["instance_count"]) == 1 && svc["autoscaling"] == nil {
				add(model.Finding{
					Severity:       "Low",
					Category:       "Availability and Resilience",
					Title:          "Production service has limited scaling and redundancy",
					ResourceType:   "app_platform_service",
					ResourceName:   fullName,
					ResourceID:     id,
					Standard:       "ISO 27001 A.5.30 / NIST PR.PT",
					Risk:           "The service may have limited ability to absorb traffic spikes or tolerate an instance-level failure.",
					BusinessImpact: "Customer-facing service may have limited resilience during failures or high load.",
					Evidence:       "instance_count=1 and autoscaling not configured",
					Recommendation: "Review expected traffic patterns, configure autoscaling where appropriate, and keep CPU/memory alerts enabled.",
					Remediation:    "Configure autoscaling where appropriate, or document why a single-instance setup is acceptable for this service.",
					Validation:     "Confirm autoscaling policy or accepted-risk exception exists.",
				})
			}
		}
	}
}

func obj(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	if m == nil {
		return map[string]interface{}{}
	}
	return m
}

func arr(v interface{}) []map[string]interface{} {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

func anySlice(v interface{}) []interface{} {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	return items
}

func str(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func boolVal(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func numeric(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(x)
		return i
	default:
		return 0
	}
}

func idString(v interface{}) string {
	if v == nil {
		return ""
	}
	if f, ok := v.(float64); ok {
		return strconv.FormatInt(int64(f), 10)
	}
	return strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%v", v), ".0"), ".000000")
}

func sliceStrings(v interface{}) []string {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, item := range items {
		out = append(out, fmt.Sprintf("%v", item))
	}
	return out
}

func hasString(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(item, needle) {
			return true
		}
	}
	return false
}

func dropletPublicIPv4(d map[string]interface{}) string {
	networks := obj(d["networks"])
	for _, net := range arr(networks["v4"]) {
		if str(net, "type") == "public" {
			return str(net, "ip_address")
		}
	}
	return ""
}

func firewallCoverage(firewalls []map[string]interface{}) (map[string]bool, map[string]bool) {
	droplets := map[string]bool{}
	tags := map[string]bool{}

	for _, fw := range firewalls {
		for _, id := range anySlice(fw["droplet_ids"]) {
			droplets[idString(id)] = true
		}
		for _, tag := range sliceStrings(fw["tags"]) {
			tags[tag] = true
		}
	}

	return droplets, tags
}

func isDropletProtected(d map[string]interface{}, dropletIDs map[string]bool, tags map[string]bool) bool {
	if dropletIDs[idString(d["id"])] {
		return true
	}
	for _, tag := range sliceStrings(d["tags"]) {
		if tags[tag] {
			return true
		}
	}
	return false
}

func isWorldOpen(rule map[string]interface{}) bool {
	sources := obj(rule["sources"])
	for _, addr := range sliceStrings(sources["addresses"]) {
		if addr == "0.0.0.0/0" || addr == "::/0" {
			return true
		}
	}
	return false
}

func severityForOpenPort(proto, ports string) string {
	p := strings.TrimSpace(strings.ToLower(ports))

	if proto == "icmp" {
		return "Low"
	}
	if p == "" || p == "all" || p == "1-65535" || p == "0-65535" {
		return "Critical"
	}

	for _, sensitive := range []string{"22", "3389", "3306", "5432", "6379", "27017", "9200", "5601", "6443"} {
		if portMatches(p, sensitive) {
			return "High"
		}
	}

	return ""
}

func portMatches(expr, port string) bool {
	for _, part := range strings.Split(expr, ",") {
		part = strings.TrimSpace(part)
		if part == port {
			return true
		}
		if strings.Contains(part, "-") {
			var a, b int
			var x int
			fmt.Sscanf(part, "%d-%d", &a, &b)
			fmt.Sscanf(port, "%d", &x)
			if a <= x && x <= b {
				return true
			}
		}
	}
	return false
}

func hasHTTPForwarding(lb map[string]interface{}) bool {
	for _, r := range arr(lb["forwarding_rules"]) {
		if idString(r["entry_port"]) == "80" || str(r, "entry_protocol") == "http" {
			return true
		}
	}
	return false
}

func isRetiredImage(d map[string]interface{}) bool {
	img := obj(d["image"])
	status := strings.ToLower(str(img, "status"))
	name := strings.ToLower(str(img, "slug") + " " + str(img, "name"))
	return strings.Contains(status, "retired") || strings.Contains(status, "deprecated") || strings.Contains(name, "retired")
}

func isProd(name string, tags []string) bool {
	v := strings.ToLower(name + " " + strings.Join(tags, " "))
	return strings.Contains(v, "prod") || strings.Contains(v, "production")
}

func defaultPriority(sev string) string {
	switch sev {
	case "Critical":
		return "P0"
	case "High":
		return "P1"
	case "Medium":
		return "P2"
	case "Low":
		return "P3"
	default:
		return "P4"
	}
}

func defaultTimeline(sev string) string {
	switch sev {
	case "Critical":
		return "Immediate / today"
	case "High":
		return "Short term / within 1 week"
	case "Medium":
		return "Medium term / within 1 month"
	case "Low":
		return "Ongoing / backlog"
	default:
		return "Informational"
	}
}

func sevRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

var uriSecretRe = regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis|amqp|smtp|https?)://[^\s:@]+:[^\s@]+@`)

func findSecretPaths(v interface{}) []string {
	var paths []string
	walkSecrets("resources", v, &paths)
	return paths
}

func walkSecrets(path string, v interface{}, paths *[]string) {
	if len(*paths) > 50 {
		return
	}

	switch x := v.(type) {
	case map[string]interface{}:
		for k, vv := range x {
			// Internal full-account copies are used only for hybrid correlation.
			// They must not generate client-scope findings.
			if strings.HasPrefix(k, "_all_") {
				continue
			}

			child := path + "." + k
			if s, ok := vv.(string); ok && looksSecret(k, s) {
				*paths = append(*paths, child)
				continue
			}
			walkSecrets(child, vv, paths)
		}
	case []interface{}:
		for i, vv := range x {
			walkSecrets(fmt.Sprintf("%s[%d]", path, i), vv, paths)
		}
	}
}

func looksSecret(key, value string) bool {
	k := strings.ToLower(key)
	v := strings.TrimSpace(value)

	if v == "" || strings.HasPrefix(v, "EV[") || strings.HasPrefix(v, "${") {
		return false
	}

	if uriSecretRe.MatchString(v) {
		return true
	}

	if strings.Contains(k, "password") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "private_key") ||
		strings.Contains(k, "access_token") ||
		strings.Contains(k, "auth_token") ||
		strings.Contains(k, "service_role") {
		return len(v) >= 8
	}

	if strings.Contains(k, "dsn") && strings.Contains(v, "@") {
		return true
	}

	return false
}
