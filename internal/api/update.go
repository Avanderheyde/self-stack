package api

import (
	"net/http"

	"github.com/selfstack/selfstack/internal/selfupdate"
)

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	release, err := selfupdate.FetchLatestRelease()
	if err != nil {
		jsonError(w, 502, "failed to check for updates")
		return
	}

	latest := selfupdate.LatestVersion(release)
	available := latest != s.version && s.version != "dev"

	jsonResponse(w, 200, map[string]any{
		"current":   s.version,
		"latest":    latest,
		"available": available,
	})
}

func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	release, err := selfupdate.FetchLatestRelease()
	if err != nil {
		jsonError(w, 502, "failed to check for updates")
		return
	}

	downloadURL, err := selfupdate.FindAssetURL(release)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}

	if err := selfupdate.DownloadAndReplace(downloadURL); err != nil {
		jsonError(w, 500, "update failed: "+err.Error())
		return
	}

	latest := selfupdate.LatestVersion(release)
	jsonResponse(w, 200, map[string]any{
		"updated": true,
		"version": latest,
	})
}
