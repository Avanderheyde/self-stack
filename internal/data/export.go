package data

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/selfstack/selfstack/internal/config"
)

func Export(ctx context.Context, appName string) (string, error) {
	exportDir := config.DataDir()
	os.MkdirAll(exportDir, 0755)

	timestamp := time.Now().Format("2006-01-02-150405")
	outFile := filepath.Join(exportDir, fmt.Sprintf("%s-%s.tar.gz", appName, timestamp))

	volumeName := fmt.Sprintf("selfstack-%s_data", appName)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volumeName+":/data",
		"-v", exportDir+":/backup",
		"alpine",
		"tar", "czf", fmt.Sprintf("/backup/%s-%s.tar.gz", appName, timestamp), "-C", "/data", ".",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("export: %s: %w", string(out), err)
	}
	return outFile, nil
}

func Import(ctx context.Context, appName, archivePath string) error {
	absPath, err := filepath.Abs(archivePath)
	if err != nil {
		return err
	}
	volumeName := fmt.Sprintf("selfstack-%s_data", appName)
	cmd := exec.CommandContext(ctx, "docker", "run", "--rm",
		"-v", volumeName+":/data",
		"-v", filepath.Dir(absPath)+":/backup",
		"alpine",
		"sh", "-c", fmt.Sprintf("tar xzf /backup/%s -C /data", filepath.Base(absPath)),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("import: %s: %w", string(out), err)
	}
	return nil
}
