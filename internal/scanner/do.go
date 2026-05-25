package scanner

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"infra-audit/internal/doapi"
	"infra-audit/internal/model"
)

type DOScanOptions struct {
	ProjectID     string
	ProjectName   string
	ScopeMode     string
	SpacesBuckets string
}

type ProjectSummary struct {
	ID            string
	Name          string
	Description   string
	ResourceCount int
}

type doEndpoint struct {
	Name string
	Path string
	Key  string
	List bool
}

func ListDigitalOceanProjects(token string) ([]ProjectSummary, error) {
	if token == "" {
		return nil, fmt.Errorf("DIGITALOCEAN_TOKEN / DO_TOKEN is empty")
	}

	c := doapi.New(token)
	projects, err := c.GetList("/v2/projects", "projects")
	if err != nil {
		return nil, err
	}

	var out []ProjectSummary
	for _, raw := range projects {
		p, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		id := str(p, "id")
		name := str(p, "name")
		desc := str(p, "description")

		count := 0
		if id != "" {
			res, err := c.GetList("/v2/projects/"+url.PathEscape(id)+"/resources", "resources")
			if err == nil {
				count = len(res)
			}
		}

		out = append(out, ProjectSummary{
			ID:            id,
			Name:          name,
			Description:   desc,
			ResourceCount: count,
		})
	}

	return out, nil
}

func ScanDigitalOcean(clientName string, token string) (model.Inventory, error) {
	return ScanDigitalOceanWithOptions(clientName, token, DOScanOptions{})
}

func ScanDigitalOceanWithOptions(clientName string, token string, opts DOScanOptions) (model.Inventory, error) {
	if token == "" {
		return model.Inventory{}, fmt.Errorf("DIGITALOCEAN_TOKEN / DO_TOKEN is empty")
	}

	c := doapi.New(token)
	inv := model.Inventory{
		Client:      clientName,
		Provider:    "DigitalOcean",
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Scope: []string{
			"account",
			"projects",
			"apps",
			"apps_detailed",
			"droplets",
			"firewalls",
			"load_balancers",
			"databases",
			"database_firewall_rules",
			"kubernetes_clusters",
			"vpcs",
			"volumes",
			"domains",
			"domain_records",
			"ssh_keys",
			"snapshots",
			"reserved_ips",
			"cdn_endpoints",
			"spaces",
			"container_registry",
		},
		Resources: map[string]interface{}{},
	}

	endpoints := []doEndpoint{
		{Name: "account", Path: "/v2/account", Key: "account", List: false},
		{Name: "projects", Path: "/v2/projects", Key: "projects", List: true},
		{Name: "apps", Path: "/v2/apps", Key: "apps", List: true},
		{Name: "droplets", Path: "/v2/droplets", Key: "droplets", List: true},
		{Name: "firewalls", Path: "/v2/firewalls", Key: "firewalls", List: true},
		{Name: "load_balancers", Path: "/v2/load_balancers", Key: "load_balancers", List: true},
		{Name: "databases", Path: "/v2/databases", Key: "databases", List: true},
		{Name: "kubernetes_clusters", Path: "/v2/kubernetes/clusters", Key: "kubernetes_clusters", List: true},
		{Name: "vpcs", Path: "/v2/vpcs", Key: "vpcs", List: true},
		{Name: "volumes", Path: "/v2/volumes", Key: "volumes", List: true},
		{Name: "domains", Path: "/v2/domains", Key: "domains", List: true},
		{Name: "ssh_keys", Path: "/v2/account/keys", Key: "ssh_keys", List: true},
		{Name: "snapshots", Path: "/v2/snapshots", Key: "snapshots", List: true},
		{Name: "reserved_ips", Path: "/v2/reserved_ips", Key: "reserved_ips", List: true},
		{Name: "cdn_endpoints", Path: "/v2/cdn/endpoints", Key: "endpoints", List: true},
	}

	for _, ep := range endpoints {
		if ep.List {
			items, err := c.GetList(ep.Path, ep.Key)
			if err != nil {
				inv.Errors = append(inv.Errors, fmt.Sprintf("%s: %v", ep.Name, err))
				inv.Resources[ep.Name] = []interface{}{}
				continue
			}
			inv.Resources[ep.Name] = items
		} else {
			obj, err := c.GetObject(ep.Path, ep.Key)
			if err != nil {
				inv.Errors = append(inv.Errors, fmt.Sprintf("%s: %v", ep.Name, err))
				inv.Resources[ep.Name] = map[string]interface{}{}
				continue
			}
			inv.Resources[ep.Name] = obj
		}
	}

	collectAppDetails(c, &inv)
	collectDomainRecords(c, &inv)
	collectDatabaseFirewalls(c, &inv)
	collectSpacesLite(&inv, opts.SpacesBuckets)
	collectContainerRegistry(c, &inv)

	preserveFullAccountLists(&inv)

	scopeMode := normalizeScopeMode(opts.ScopeMode)

	if scopeMode != "account" && (strings.TrimSpace(opts.ProjectID) != "" || strings.TrimSpace(opts.ProjectName) != "") {
		if err := applyProjectFilter(c, &inv, opts); err != nil {
			inv.Errors = append(inv.Errors, "project_filter: "+err.Error())
		}

		if scopeMode == "hybrid" {
			applyHybridRelatedResources(&inv)
		}
	}

	if scopeMode == "account" {
		inv.Scope = append(inv.Scope, "scope_mode=account_full_scan")
	}

	restoreSpacesAfterFiltering(&inv)

	return inv, nil
}

func restoreSpacesAfterFiltering(inv *model.Inventory) {
	if raw, ok := inv.Resources["_all_spaces"]; ok && raw != nil {
		inv.Resources["spaces"] = raw
	}
}

func preserveFullAccountLists(inv *model.Inventory) {
	for _, key := range []string{
		"apps",
		"apps_detailed",
		"droplets",
		"firewalls",
		"load_balancers",
		"databases",
		"kubernetes_clusters",
		"vpcs",
		"volumes",
		"domains",
		"domain_records",
		"reserved_ips",
		"snapshots",
		"cdn_endpoints",
		"spaces",
		"container_registry",
	} {
		inv.Resources["_all_"+key] = inv.Resources[key]
	}
}

func normalizeScopeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "project"
	}
	return mode
}

func applyHybridRelatedResources(inv *model.Inventory) {
	selectedText := strings.ToLower(fmt.Sprintf("%v", inv.Resources["selected_project"]) + " " + fmt.Sprintf("%v", inv.Resources["apps"]) + " " + fmt.Sprintf("%v", inv.Resources["apps_detailed"]) + " " + fmt.Sprintf("%v", inv.Resources["databases"]))

	keywords := relatedKeywords(selectedText)

	for _, key := range []string{"droplets", "firewalls", "vpcs", "domains", "reserved_ips", "volumes", "snapshots"} {
		current := asInterfaceSlice(inv.Resources[key])
		if len(current) > 0 {
			continue
		}

		// Original full-account list was already overwritten by strict filtering,
		// so hybrid enrichment requires the full raw list to be saved first.
		// If not available, keep current list empty.
		rawKey := "_all_" + key
		raw := asInterfaceSlice(inv.Resources[rawKey])
		if len(raw) == 0 {
			continue
		}

		var related []interface{}
		for _, item := range raw {
			if isRelatedResource(item, keywords, selectedText) {
				related = append(related, item)
			}
		}

		inv.Resources[key] = related
	}

	inv.Scope = append(inv.Scope, "scope_mode=hybrid_related_resources")
}

func relatedKeywords(text string) []string {
	base := []string{
		"prod",
		"dev",
	}

	seen := map[string]bool{}
	var out []string

	for _, x := range base {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}

	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		token = strings.TrimSpace(token)
		if len(token) < 3 {
			continue
		}
		if token == "digitalocean" || token == "com" || token == "app" || token == "api" {
			continue
		}
		if !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
	}

	return out
}

func isRelatedResource(item interface{}, keywords []string, selectedText string) bool {
	text := strings.ToLower(fmt.Sprintf("%v", item))

	for _, kw := range keywords {
		if kw != "" && strings.Contains(text, kw) {
			return true
		}
	}

	// Reserved IP may be related if referenced by selected app/database evidence.
	if m, ok := item.(map[string]interface{}); ok {
		ip := str(m, "ip")
		if ip != "" && strings.Contains(selectedText, strings.ToLower(ip)) {
			return true
		}
	}

	return false
}

func asInterfaceSlice(v interface{}) []interface{} {
	items, _ := v.([]interface{})
	return items
}

func applyProjectFilter(c *doapi.Client, inv *model.Inventory, opts DOScanOptions) error {
	project, err := selectProject(inv, opts)
	if err != nil {
		return err
	}

	projectID := str(project, "id")
	if projectID == "" {
		return fmt.Errorf("selected project has empty id")
	}

	rawResources, err := c.GetList("/v2/projects/"+url.PathEscape(projectID)+"/resources", "resources")
	if err != nil {
		return err
	}

	allowed := map[string]bool{}
	for _, raw := range rawResources {
		if m, ok := raw.(map[string]interface{}); ok {
			urn := str(m, "urn")
			if urn != "" {
				allowed[urn] = true
			}
		}
	}

	inv.Scope = append(inv.Scope, "project_filter="+str(project, "name")+" ("+projectID+")")
	inv.Resources["selected_project"] = project
	inv.Resources["project_resources"] = rawResources
	inv.Resources["projects"] = []interface{}{project}

	for _, key := range []string{
		"apps",
		"apps_detailed",
		"droplets",
		"firewalls",
		"load_balancers",
		"databases",
		"kubernetes_clusters",
		"vpcs",
		"volumes",
		"domains",
		"reserved_ips",
		"cdn_endpoints",
	} {
		inv.Resources[key] = filterResourceList(key, inv.Resources[key], allowed)
	}

	keepDBNames := map[string]bool{}
	for _, db := range arr(inv.Resources["databases"]) {
		keepDBNames[str(db, "name")] = true
	}
	inv.Resources["database_firewall_rules"] = filterNamedMap(inv.Resources["database_firewall_rules"], keepDBNames)

	keepDomains := map[string]bool{}
	for _, d := range arr(inv.Resources["domains"]) {
		keepDomains[str(d, "name")] = true
	}
	inv.Resources["domain_records"] = filterNamedMap(inv.Resources["domain_records"], keepDomains)

	return nil
}

func selectProject(inv *model.Inventory, opts DOScanOptions) (map[string]interface{}, error) {
	projects := arr(inv.Resources["projects"])
	idWant := strings.TrimSpace(opts.ProjectID)
	nameWant := strings.ToLower(strings.TrimSpace(opts.ProjectName))

	if idWant != "" {
		for _, p := range projects {
			if str(p, "id") == idWant {
				return p, nil
			}
		}
		return nil, fmt.Errorf("project id not found: %s", idWant)
	}

	if nameWant != "" {
		for _, p := range projects {
			if strings.ToLower(str(p, "name")) == nameWant {
				return p, nil
			}
		}
		for _, p := range projects {
			if strings.Contains(strings.ToLower(str(p, "name")), nameWant) {
				return p, nil
			}
		}
		return nil, fmt.Errorf("project name not found: %s", opts.ProjectName)
	}

	return nil, fmt.Errorf("project id/name is empty")
}

func filterResourceList(resourceKey string, v interface{}, allowed map[string]bool) []interface{} {
	items, ok := v.([]interface{})
	if !ok {
		return []interface{}{}
	}

	var out []interface{}
	for _, raw := range items {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		for _, candidate := range resourceURNCandidates(resourceKey, m) {
			if allowed[candidate] {
				out = append(out, raw)
				break
			}
		}
	}

	return out
}

func filterNamedMap(v interface{}, keep map[string]bool) map[string]interface{} {
	src, ok := v.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	out := map[string]interface{}{}
	for name, value := range src {
		if keep[name] {
			out[name] = value
		}
	}

	return out
}

func resourceURNCandidates(resourceKey string, m map[string]interface{}) []string {
	var out []string

	if urn := str(m, "urn"); urn != "" {
		out = append(out, urn)
	}

	id := str(m, "id")
	name := str(m, "name")
	ip := str(m, "ip")

	switch resourceKey {
	case "droplets":
		out = append(out, "do:droplet:"+id)
	case "apps", "apps_detailed":
		out = append(out, "do:app:"+id)
	case "databases":
		out = append(out, "do:dbaas:"+id, "do:database:"+id)
	case "firewalls":
		out = append(out, "do:firewall:"+id)
	case "load_balancers":
		out = append(out, "do:loadbalancer:"+id, "do:load_balancer:"+id)
	case "kubernetes_clusters":
		out = append(out, "do:kubernetes:"+id)
	case "vpcs":
		out = append(out, "do:vpc:"+id)
	case "volumes":
		out = append(out, "do:volume:"+id)
	case "domains":
		out = append(out, "do:domain:"+name)
	case "reserved_ips":
		out = append(out, "do:reservedip:"+ip, "do:reserved_ip:"+ip)
	case "cdn_endpoints":
		out = append(out, "do:cdn_endpoint:"+id)
	}

	var clean []string
	for _, item := range out {
		item = strings.TrimSpace(item)
		if item != "" && !strings.HasSuffix(item, ":") {
			clean = append(clean, item)
		}
	}

	return clean
}

func collectAppDetails(c *doapi.Client, inv *model.Inventory) {
	var detailed []interface{}

	for _, app := range arr(inv.Resources["apps"]) {
		id := str(app, "id")
		if id == "" {
			continue
		}

		obj, err := c.GetObject("/v2/apps/"+url.PathEscape(id), "app")
		if err != nil {
			inv.Errors = append(inv.Errors, fmt.Sprintf("app_detail:%s: %v", id, err))
			continue
		}

		detailed = append(detailed, obj)
	}

	inv.Resources["apps_detailed"] = detailed
}

func collectDomainRecords(c *doapi.Client, inv *model.Inventory) {
	records := map[string]interface{}{}

	for _, d := range arr(inv.Resources["domains"]) {
		name := str(d, "name")
		if name == "" {
			continue
		}

		items, err := c.GetList("/v2/domains/"+url.PathEscape(name)+"/records", "domain_records")
		if err != nil {
			inv.Errors = append(inv.Errors, fmt.Sprintf("domain_records:%s: %v", name, err))
			continue
		}

		records[name] = items
	}

	inv.Resources["domain_records"] = records
}

func collectDatabaseFirewalls(c *doapi.Client, inv *model.Inventory) {
	result := map[string]interface{}{}

	for _, db := range arr(inv.Resources["databases"]) {
		id := str(db, "id")
		name := str(db, "name")
		if id == "" {
			continue
		}

		rules, err := c.GetObject("/v2/databases/"+url.PathEscape(id)+"/firewall", "rules")
		if err != nil {
			inv.Errors = append(inv.Errors, fmt.Sprintf("database_firewall:%s: %v", name, err))
			continue
		}

		result[name] = rules
	}

	inv.Resources["database_firewall_rules"] = result
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

func str(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
