package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var assets embed.FS

func Handler() http.Handler {
	dist, _ := fs.Sub(assets, "dist")
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve static files directly if path has a file extension
		if strings.Contains(r.URL.Path, ".") {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for all other paths
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
