package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"infra-audit/internal/model"
)

func checkContainerRegistry(inv model.Inventory, add func(model.Finding)) {
	cr := obj(inv.Resources["container_registry"])
	if len(cr) == 0 {
		return
	}

	registry := obj(cr["registry"])
	subscription := obj(cr["subscription"])
	tier := obj(subscription["tier"])

	registryName := str(registry, "name")
	if registryName == "" {
		return
	}

	storageBytes := int64(regNum(registry["storage_usage_bytes"]))
	includedBytes := int64(regNum(tier["included_storage_bytes"]))
	repoLimit := int(regNum(tier["included_repositories"]))

	repos := arr(cr["repositories"])
	manifestsByRepo := obj(cr["repository_manifests"])
	gcs := arr(cr["garbage_collections"])

	if includedBytes > 0 && storageBytes > includedBytes {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Registry Hygiene",
			Title:          "Container Registry storage exceeds included plan capacity",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.8.8 / CIS Control 7 / NIST PR.IP",
			Risk:           "The registry is using more storage than the included plan capacity. This is mainly an operational and cost-control issue, but it can also indicate that old images and untagged manifests are not being cleaned up.",
			BusinessImpact: "Storage growth can increase cost and make it harder to identify which images are current and safe to deploy.",
			Evidence:       fmt.Sprintf("storage_usage=%s included_storage=%s", humanBytes(storageBytes), humanBytes(includedBytes)),
			Recommendation: "Review image retention and cleanup practices for the registry.",
			Remediation:    "Delete obsolete tags/manifests, run garbage collection, and define a retention policy for old CI/CD images.",
			Validation:     "Re-run the scan and confirm storage usage is within the expected range or documented as accepted cost.",
			Timeline:       "Ongoing / backlog",
		})
	}

	if repoLimit > 0 && len(repos) > repoLimit {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Registry Hygiene",
			Title:          "Container Registry repository count exceeds included plan limit",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.5.9 / CIS Control 1",
			Risk:           "The registry contains more repositories than the included plan limit. This may indicate uncontrolled repository growth or missing cleanup ownership.",
			BusinessImpact: "Repository sprawl can make CI/CD ownership, cleanup, and image provenance harder to manage.",
			Evidence:       fmt.Sprintf("repositories=%d included_repositories=%d", len(repos), repoLimit),
			Recommendation: "Review repository ownership and remove obsolete repositories.",
			Remediation:    "Define repository ownership, archive unused images, and document which repositories are still deployed.",
			Validation:     "Re-run the scan and confirm obsolete repositories were removed or documented.",
			Timeline:       "Ongoing / backlog",
		})
	}

	if len(gcs) == 0 && storageBytes > 0 {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Registry Hygiene",
			Title:          "Container Registry has no recorded garbage collection runs",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.8.8 / CIS Control 7",
			Risk:           "Deleted or untagged image layers may continue consuming storage until garbage collection runs.",
			BusinessImpact: "Registry storage can grow over time and make old image cleanup ineffective.",
			Evidence:       "garbage_collections list is empty or unavailable.",
			Recommendation: "Run garbage collection after deleting obsolete manifests or tags.",
			Remediation:    "Schedule registry cleanup and garbage collection as part of CI/CD maintenance.",
			Validation:     "Re-run the scan and confirm at least one garbage collection record exists after cleanup.",
			Timeline:       "Ongoing / backlog",
		})
	}

	staleCount, untaggedCount, latestCount := registryManifestCounts(cr, manifestsByRepo)

	if untaggedCount > 0 {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Registry Hygiene",
			Title:          "Container Registry contains untagged image manifests",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.8.8 / CIS Control 7",
			Risk:           "Untagged manifests are usually leftover artifacts from image churn and may remain after tag deletion.",
			BusinessImpact: "Registry storage can grow and cleanup may not reclaim space unless garbage collection is run.",
			Evidence:       fmt.Sprintf("untagged_manifests=%d", untaggedCount),
			Recommendation: "Remove obsolete manifests and run garbage collection.",
			Remediation:    "Delete old untagged manifests where safe and start registry garbage collection.",
			Validation:     "Re-run the scan and confirm untagged manifests are reduced or documented.",
			Timeline:       "Ongoing / backlog",
		})
	}

	if staleCount > 0 {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Image Hygiene",
			Title:          "Container Registry contains stale image manifests",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.8.8 / CIS Control 7 / NIST PR.IP",
			Risk:           "Old container images may contain outdated dependencies or base images. The registry should not become a long-term archive for deployable artifacts without retention rules.",
			BusinessImpact: "Teams may accidentally redeploy outdated images or spend time maintaining unused artifacts.",
			Evidence:       fmt.Sprintf("manifests_older_than_180_days=%d", staleCount),
			Recommendation: "Define image retention rules and remove images that are no longer deployable or needed for rollback.",
			Remediation:    "Keep a limited rollback window, delete obsolete manifests, and run garbage collection.",
			Validation:     "Re-run the scan and confirm stale manifests were removed or documented as required rollback artifacts.",
			Timeline:       "Ongoing / backlog",
		})
	}

	if latestCount > 0 {
		add(model.Finding{
			Severity:       "Low",
			Category:       "Container Image Deployment Control",
			Title:          "Container Registry uses mutable latest tags",
			ResourceType:   "container_registry",
			ResourceName:   registryName,
			Standard:       "ISO 27001 A.8.32 / NIST PR.IP",
			Risk:           "The tag 'latest' is mutable and does not uniquely identify the image version. If used in deployment configuration, it can reduce traceability and rollback confidence.",
			BusinessImpact: "It may be harder to prove exactly which image version was deployed during an incident or rollback.",
			Evidence:       fmt.Sprintf("manifests_with_latest_tag=%d", latestCount),
			Recommendation: "Use immutable version tags or digests for production deployments.",
			Remediation:    "Update deployment configuration to pin images by release tag or digest and keep 'latest' only for non-production convenience if needed.",
			Validation:     "Confirm production deployment manifests use immutable tags or image digests.",
			Timeline:       "Ongoing / backlog",
		})
	}
}

func stringSlice(v interface{}) []string {
	var out []string

	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, t...)
	}

	return out
}

func manifestIsStale(m map[string]interface{}, days int) bool {
	for _, key := range []string{"updated_at", "created_at"} {
		raw := strings.TrimSpace(fmt.Sprintf("%v", m[key]))
		if raw == "" || raw == "<nil>" {
			continue
		}

		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			continue
		}

		return time.Since(t) > time.Duration(days)*24*time.Hour
	}

	return false
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}

	units := []string{"KiB", "MiB", "GiB", "TiB"}
	f := float64(n)

	for _, unit := range units {
		f = f / 1024
		if f < 1024 {
			return fmt.Sprintf("%.2f %s", f, unit)
		}
	}

	return fmt.Sprintf("%.2f PiB", f/1024)
}

func regNum(v interface{}) float64 {
	switch t := v.(type) {
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case float64:
		return t
	case float32:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

func registryManifestCounts(cr map[string]interface{}, manifestsByRepo map[string]interface{}) (int, int, int) {
	summary := obj(cr["repository_manifest_summary"])

	if len(summary) > 0 {
		stale := 0
		untagged := 0
		latest := 0

		for _, raw := range summary {
			row := obj(raw)
			stale += int(regNum(row["stale_manifests"]))
			untagged += int(regNum(row["untagged_manifests"]))
			latest += int(regNum(row["latest_tag_manifests"]))
		}

		return stale, untagged, latest
	}

	stale := 0
	untagged := 0
	latest := 0

	for _, raw := range manifestsByRepo {
		manifests, ok := raw.([]interface{})
		if !ok {
			continue
		}

		for _, item := range manifests {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			tags := stringSlice(m["tags"])
			if len(tags) == 0 {
				untagged++
			}

			for _, tag := range tags {
				if tag == "latest" {
					latest++
				}
			}

			if manifestIsStale(m, 180) {
				stale++
			}
		}
	}

	return stale, untagged, latest
}
