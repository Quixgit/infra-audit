package scanner

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"infra-audit/internal/doapi"
	"infra-audit/internal/model"
)

func collectContainerRegistry(c *doapi.Client, inv *model.Inventory) {
	inv.Scope = append(inv.Scope, "container_registry")

	result := map[string]interface{}{
		"registry":             map[string]interface{}{},
		"subscription":         map[string]interface{}{},
		"repositories":         []interface{}{},
		"repository_manifests": map[string]interface{}{},
		"garbage_collections":  []interface{}{},
		"errors":               []string{},
		"collected_at":         time.Now().UTC().Format(time.RFC3339),
	}

	registryRaw, err := c.GetObject("/v2/registry", "registry")
	if err != nil {
		result["status"] = "skipped_or_not_available"
		result["errors"] = appendString(result["errors"], fmt.Sprintf("registry: %v", err))
		inv.Resources["container_registry"] = result
		return
	}

	registry, _ := registryRaw.(map[string]interface{})

	result["status"] = "completed"
	result["registry"] = registry

	name := fmt.Sprintf("%v", registry["name"])
	if name == "" || name == "<nil>" {
		inv.Resources["container_registry"] = result
		return
	}

	subscription, err := c.GetObject("/v2/registry/subscription", "subscription")
	if err != nil {
		result["errors"] = appendString(result["errors"], fmt.Sprintf("subscription: %v", err))
	} else {
		result["subscription"] = subscription
	}

	repos, err := c.GetList("/v2/registry/"+url.PathEscape(name)+"/repositoriesV2", "repositories")
	if err != nil {
		result["errors"] = appendString(result["errors"], fmt.Sprintf("repositories: %v", err))
	} else {
		result["repositories"] = repos
	}

	manifestMap := map[string]interface{}{}
	manifestSummary := map[string]interface{}{}
	for _, raw := range repos {
		repo, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		repoName := fmt.Sprintf("%v", repo["name"])
		if repoName == "" {
			repoName = fmt.Sprintf("%v", repo["repository"])
		}
		if repoName == "" {
			continue
		}

		manifests, err := c.GetList(
			"/v2/registry/"+url.PathEscape(name)+"/repositories/"+url.PathEscape(repoName)+"/digests",
			"manifests",
		)
		if err != nil {
			result["errors"] = appendString(result["errors"], fmt.Sprintf("manifests:%s: %v", repoName, err))
			continue
		}

		cleanManifests := sanitizeRegistryManifests(manifests)
		manifestSummary[repoName] = summarizeRegistryManifests(cleanManifests, 180)
		manifestMap[repoName] = limitRegistryItems(cleanManifests, 10)
	}
	result["repository_manifests"] = manifestMap
	result["repository_manifest_summary"] = manifestSummary

	gcs, err := c.GetList("/v2/registry/"+url.PathEscape(name)+"/garbage-collections", "garbage_collections")
	if err != nil {
		result["errors"] = appendString(result["errors"], fmt.Sprintf("garbage_collections: %v", err))
	} else {
		result["garbage_collections_total"] = len(gcs)
		result["garbage_collections"] = limitRegistryItems(gcs, 10)
	}

	inv.Resources["container_registry"] = result
}

func appendString(v interface{}, s string) []string {
	var out []string

	if items, ok := v.([]string); ok {
		out = append(out, items...)
	}

	out = append(out, s)
	return out
}

func sanitizeRegistryManifests(items []interface{}) []interface{} {
	out := make([]interface{}, 0, len(items))

	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			out = append(out, item)
			continue
		}

		clean := map[string]interface{}{}

		for k, v := range m {
			// Blobs are very large and not needed for report-level evidence.
			if k == "blobs" {
				continue
			}
			clean[k] = v
		}

		out = append(out, clean)
	}

	return out
}

func limitRegistryItems(items []interface{}, max int) []interface{} {
	if max <= 0 || len(items) <= max {
		return items
	}
	return items[:max]
}

func summarizeRegistryManifests(items []interface{}, staleDays int) map[string]interface{} {
	total := 0
	untagged := 0
	latest := 0
	stale := 0
	var totalSize int64
	var totalCompressed int64
	var oldest string
	var newest string

	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		total++

		tags := registryTags(m["tags"])
		if len(tags) == 0 {
			untagged++
		}

		for _, tag := range tags {
			if tag == "latest" {
				latest++
			}
		}

		totalSize += int64(registryNum(m["size_bytes"]))
		totalCompressed += int64(registryNum(m["compressed_size_bytes"]))

		rawUpdated := strings.TrimSpace(fmt.Sprintf("%v", m["updated_at"]))
		if rawUpdated != "" && rawUpdated != "<nil>" {
			if newest == "" || rawUpdated > newest {
				newest = rawUpdated
			}
			if oldest == "" || rawUpdated < oldest {
				oldest = rawUpdated
			}

			if t, err := time.Parse(time.RFC3339, rawUpdated); err == nil {
				if time.Since(t) > time.Duration(staleDays)*24*time.Hour {
					stale++
				}
			}
		}
	}

	return map[string]interface{}{
		"total_manifests":            total,
		"untagged_manifests":         untagged,
		"latest_tag_manifests":       latest,
		"stale_manifests":            stale,
		"stale_threshold_days":       staleDays,
		"total_size_bytes":           totalSize,
		"total_compressed_bytes":     totalCompressed,
		"oldest_manifest_updated_at": oldest,
		"newest_manifest_updated_at": newest,
		"stored_manifest_examples":   minInt(total, 10),
	}
}

func registryTags(v interface{}) []string {
	var out []string

	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, t...)
	}

	return out
}

func registryNum(v interface{}) float64 {
	switch t := v.(type) {
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case float64:
		return t
	case float32:
		return float64(t)
	default:
		return 0
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
