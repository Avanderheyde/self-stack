package cmd

import (
	"fmt"

	"github.com/selfstack/selfstack/internal/selfupdate"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update SelfStack to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: %s\n", Version)

		release, err := selfupdate.FetchLatestRelease()
		if err != nil {
			return fmt.Errorf("check for updates: %w", err)
		}

		latest := selfupdate.LatestVersion(release)
		if latest == Version {
			fmt.Println("Already up to date.")
			return nil
		}

		downloadURL, err := selfupdate.FindAssetURL(release)
		if err != nil {
			return err
		}

		fmt.Printf("Downloading %s...\n", release.TagName)
		if err := selfupdate.DownloadAndReplace(downloadURL); err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		fmt.Printf("Updated selfstack from v%s to %s\n", Version, release.TagName)
		return nil
	},
}

func init() { rootCmd.AddCommand(updateCmd) }
