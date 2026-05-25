package scanner

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"infra-audit/internal/model"
)

type SpacesBucketSpec struct {
	Name        string
	Region      string
	Sensitivity string
}

func collectSpacesLite(inv *model.Inventory, spec string) {
	buckets := parseSpacesBuckets(spec)

	inv.Scope = append(inv.Scope, "spaces_public_probe")

	if len(buckets) == 0 {
		inv.Resources["spaces"] = []interface{}{}
		inv.Resources["spaces_scan_status"] = map[string]interface{}{
			"status":  "skipped",
			"message": "Spaces scan skipped. Pass --spaces-buckets \"bucket:region:sensitive,bucket2:region\" to include Spaces public-probe checks.",
		}
		return
	}

	var out []interface{}

	for _, b := range buckets {
		baseURL := fmt.Sprintf("https://%s.%s.digitaloceanspaces.com", b.Name, b.Region)
		listURL := baseURL + "/?list-type=2&max-keys=1"

		listProbe := httpProbe(listURL)
		rootProbe := httpProbe(baseURL + "/")

		checks := map[string]interface{}{
			"scan_mode":                     "public_probe_only",
			"public_bucket_listing":         listProbe.StatusCode == 200 && strings.Contains(strings.ToLower(listProbe.BodyPrefix), "listbucketresult"),
			"public_bucket_listing_status":  listProbe.StatusCode,
			"public_root_status":            rootProbe.StatusCode,
			"public_root_accessible":        rootProbe.StatusCode == 200,
			"sensitive_bucket_name":         looksSensitiveSpaceBucket(b.Name) || strings.EqualFold(b.Sensitivity, "sensitive"),
			"acl_verified":                  false,
			"bucket_policy_verified":        false,
			"website_hosting_verified":      false,
			"cors_verified":                 false,
			"lifecycle_verified":            false,
			"versioning_verified":           false,
			"requires_spaces_access_key":    true,
			"manual_validation_required":    true,
			"manual_validation_reason":      "ACL, bucket policy, CORS, lifecycle and versioning require a Spaces S3-compatible access key/secret.",
			"unauthenticated_probe_summary": fmt.Sprintf("listing_status=%d root_status=%d", listProbe.StatusCode, rootProbe.StatusCode),
		}

		out = append(out, map[string]interface{}{
			"name":        b.Name,
			"region":      b.Region,
			"endpoint":    baseURL,
			"sensitivity": b.Sensitivity,
			"checks":      checks,
		})
	}

	inv.Resources["spaces"] = out
	inv.Resources["spaces_scan_status"] = map[string]interface{}{
		"status":       "completed_public_probe_only",
		"bucket_count": len(out),
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"note":         "Public probe completed without Spaces access key. Authenticated configuration checks require Spaces key/secret.",
	}
}

func parseSpacesBuckets(spec string) []SpacesBucketSpec {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil
	}

	var out []SpacesBucketSpec

	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.Split(item, ":")
		name := ""
		region := "tor1"
		sensitivity := ""

		if len(parts) >= 1 {
			name = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
			region = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			sensitivity = strings.TrimSpace(parts[2])
		}

		if name == "" {
			continue
		}

		out = append(out, SpacesBucketSpec{
			Name:        name,
			Region:      region,
			Sensitivity: sensitivity,
		})
	}

	return out
}

type probeResult struct {
	StatusCode int
	BodyPrefix string
	Error      string
}

func httpProbe(url string) probeResult {
	client := &http.Client{Timeout: 7 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return probeResult{StatusCode: 0, Error: err.Error()}
	}

	req.Header.Set("User-Agent", "infra-audit-public-probe/1.0")

	res, err := client.Do(req)
	if err != nil {
		return probeResult{StatusCode: 0, Error: err.Error()}
	}
	defer res.Body.Close()

	limited := io.LimitReader(res.Body, 4096)
	body, _ := io.ReadAll(limited)

	return probeResult{
		StatusCode: res.StatusCode,
		BodyPrefix: string(body),
	}
}

func looksSensitiveSpaceBucket(name string) bool {
	n := strings.ToLower(name)

	for _, token := range []string{
		"terraform",
		"state",
		"tfstate",
		"prod",
		"production",
		"backup",
		"db",
		"database",
		"private",
		"secret",
		"log",
		"logs",
		"file",
		"files",
		"upload",
		"uploads",
		"tenant",
		"invoice",
		"document",
		"documents",
	} {
		if strings.Contains(n, token) {
			return true
		}
	}

	return false
}
