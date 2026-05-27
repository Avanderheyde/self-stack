package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var testCatalog = Catalog{
	Apps: []AppEntry{
		{
			Name:        "wordpress",
			DisplayName: "WordPress",
			Description: "Popular blogging platform",
			Category:    "cms",
			Repo:        "https://github.com/example/wordpress",
			Icon:        "wordpress.png",
			Verified:    true,
		},
		{
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "Self-hosted file sync and share",
			Category:    "storage",
			Repo:        "https://github.com/example/nextcloud",
			Icon:        "nextcloud.png",
			Verified:    true,
		},
		{
			Name:        "ghost",
			DisplayName: "Ghost",
			Description: "Professional publishing platform",
			Category:    "cms",
			Repo:        "https://github.com/example/ghost",
			Icon:        "ghost.png",
			Verified:    false,
		},
	},
	Categories: []string{"cms", "storage"},
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testCatalog)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	cat, err := c.Fetch()
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if len(cat.Apps) != 3 {
		t.Errorf("expected 3 apps, got %d", len(cat.Apps))
	}
	if len(cat.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cat.Categories))
	}
	if cat.Apps[0].Name != "wordpress" {
		t.Errorf("expected first app name %q, got %q", "wordpress", cat.Apps[0].Name)
	}
	if cat.Apps[0].Verified != true {
		t.Errorf("expected first app to be verified")
	}

	// Verify cache is populated after Fetch
	cached := c.Cached()
	if cached == nil {
		t.Fatal("expected cache to be populated after Fetch")
	}
	if len(cached.Apps) != 3 {
		t.Errorf("cached apps: expected 3, got %d", len(cached.Apps))
	}
}

func TestFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Fetch()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantLen int
		wantApp string
	}{
		{
			name:    "search by name",
			query:   "wordpress",
			wantLen: 1,
			wantApp: "wordpress",
		},
		{
			name:    "search by description",
			query:   "file sync",
			wantLen: 1,
			wantApp: "nextcloud",
		},
		{
			name:    "search by category",
			query:   "cms",
			wantLen: 2,
		},
		{
			name:    "case insensitive",
			query:   "GHOST",
			wantLen: 1,
			wantApp: "ghost",
		},
		{
			name:    "no results",
			query:   "nonexistent",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := Search(&testCatalog, tt.query)
			if len(results) != tt.wantLen {
				t.Errorf("Search(%q): got %d results, want %d", tt.query, len(results), tt.wantLen)
			}
			if tt.wantApp != "" && len(results) > 0 && results[0].Name != tt.wantApp {
				t.Errorf("Search(%q): first result %q, want %q", tt.query, results[0].Name, tt.wantApp)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(testCatalog)
	}))
	defer srv.Close()

	t.Run("found", func(t *testing.T) {
		c := NewClient(srv.URL)
		app, err := c.Lookup("nextcloud")
		if err != nil {
			t.Fatalf("Lookup() error: %v", err)
		}
		if app.Name != "nextcloud" {
			t.Errorf("expected %q, got %q", "nextcloud", app.Name)
		}
		if app.DisplayName != "Nextcloud" {
			t.Errorf("expected display name %q, got %q", "Nextcloud", app.DisplayName)
		}
	})

	t.Run("not found", func(t *testing.T) {
		c := NewClient(srv.URL)
		_, err := c.Lookup("doesnotexist")
		if err == nil {
			t.Fatal("expected error for missing app")
		}
	})

	t.Run("uses cache", func(t *testing.T) {
		c := NewClient(srv.URL)
		// First call populates cache
		_, _ = c.Fetch()
		// Close server to prove cache is used
		srv.Close()
		app, err := c.Lookup("wordpress")
		if err != nil {
			t.Fatalf("Lookup() with cache error: %v", err)
		}
		if app.Name != "wordpress" {
			t.Errorf("expected %q, got %q", "wordpress", app.Name)
		}
	})
}
