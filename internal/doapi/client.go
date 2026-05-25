package doapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	Token      string
	BaseURL    string
	HTTPClient *http.Client
}

func New(token string) *Client {
	return &Client{
		Token:   strings.TrimSpace(token),
		BaseURL: "https://api.digitalocean.com",
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) getJSON(urlOrPath string, out interface{}) error {
	url := urlOrPath
	if !strings.HasPrefix(url, "http") {
		url = c.BaseURL + urlOrPath
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "infra-audit/0.1")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("DO API %s returned HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

func (c *Client) GetObject(path string, key string) (interface{}, error) {
	var payload map[string]interface{}
	if err := c.getJSON(path, &payload); err != nil {
		return nil, err
	}
	if key == "" {
		return payload, nil
	}
	return payload[key], nil
}

func (c *Client) GetList(path string, key string) ([]interface{}, error) {
	var all []interface{}
	next := withPerPage(path)

	for next != "" {
		var payload map[string]interface{}
		if err := c.getJSON(next, &payload); err != nil {
			return all, err
		}

		items, ok := payload[key].([]interface{})
		if !ok {
			return all, fmt.Errorf("unexpected response: key %q is not array", key)
		}
		all = append(all, items...)

		next = ""
		if links, ok := payload["links"].(map[string]interface{}); ok {
			if pages, ok := links["pages"].(map[string]interface{}); ok {
				if n, ok := pages["next"].(string); ok {
					next = n
				}
			}
		}
	}

	return all, nil
}

func withPerPage(path string) string {
	if strings.Contains(path, "per_page=") {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "per_page=200"
}
