package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type AppEntry struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Repo        string `json:"repo"`
	Icon        string `json:"icon"`
	Verified    bool   `json:"verified"`
	Version     string `json:"version"`
}

type Catalog struct {
	Apps       []AppEntry `json:"apps"`
	Categories []string   `json:"categories"`
}

type Client struct {
	url     string
	client  *http.Client
	cache   *Catalog
	fetched time.Time
}

func NewClient(url string) *Client {
	return &Client{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Fetch() (*Catalog, error) {
	resp, err := c.client.Get(c.url)
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
	}
	var cat Catalog
	if err := json.NewDecoder(resp.Body).Decode(&cat); err != nil {
		return nil, fmt.Errorf("decode registry: %w", err)
	}
	c.cache = &cat
	c.fetched = time.Now()
	return &cat, nil
}

const cacheTTL = 5 * time.Minute

func (c *Client) Cached() *Catalog {
	if c.cache != nil && time.Since(c.fetched) > cacheTTL {
		return nil
	}
	return c.cache
}

func (c *Client) Lookup(name string) (*AppEntry, error) {
	cat := c.cache
	if cat == nil {
		var err error
		cat, err = c.Fetch()
		if err != nil {
			return nil, err
		}
	}
	for _, app := range cat.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, fmt.Errorf("app %q not found in registry", name)
}

func Search(cat *Catalog, query string) []AppEntry {
	q := strings.ToLower(query)
	var results []AppEntry
	for _, app := range cat.Apps {
		if strings.Contains(strings.ToLower(app.Name), q) ||
			strings.Contains(strings.ToLower(app.DisplayName), q) ||
			strings.Contains(strings.ToLower(app.Description), q) ||
			strings.Contains(strings.ToLower(app.Category), q) {
			results = append(results, app)
		}
	}
	return results
}
