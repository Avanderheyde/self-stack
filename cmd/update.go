package cmd

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubRepo = "Avanderheyde/self-stack"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update SelfStack to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: %s\n", Version)

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("check for updates: %w", err)
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		if latest == Version {
			fmt.Println("Already up to date.")
			return nil
		}

		assetName := fmt.Sprintf("selfstack_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
		var downloadURL string
		for _, a := range release.Assets {
			if a.Name == assetName {
				downloadURL = a.BrowserDownloadURL
				break
			}
		}
		if downloadURL == "" {
			return fmt.Errorf("no release binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
		}

		fmt.Printf("Downloading %s...\n", release.TagName)
		if err := downloadAndReplace(downloadURL); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Printf("Updated selfstack from v%s to %s\n", Version, release.TagName)
		return nil
	},
}

func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func downloadAndReplace(url string) error {
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

func init() { rootCmd.AddCommand(updateCmd) }
