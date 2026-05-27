package selfupdate

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const GithubRepo = "Avanderheyde/self-stack"

type GHRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []GHAsset `json:"assets"`
}

type GHAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func FetchLatestRelease() (*GHRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GithubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func DownloadAndReplace(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	// tar contains a single file: selfstack
	// Skip the 512-byte tar header to get to file content
	header := make([]byte, 512)
	if _, err := io.ReadFull(gr, header); err != nil {
		return fmt.Errorf("read tar header: %w", err)
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "selfstack-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, gr); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmp.Name(), 0755); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), exe)
}

// AssetName returns the expected release asset filename for the current platform.
func AssetName() string {
	return fmt.Sprintf("selfstack_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
}

// FindAssetURL finds the download URL for the current platform in a release.
func FindAssetURL(release *GHRelease) (string, error) {
	name := AssetName()
	for _, a := range release.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// LatestVersion returns the version string (without "v" prefix) from a release.
func LatestVersion(release *GHRelease) string {
	return strings.TrimPrefix(release.TagName, "v")
}
