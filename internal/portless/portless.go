package portless

import (
	"os/exec"
	"strconv"
	"strings"
)

// Available returns true if the portless CLI is installed.
func Available() bool {
	_, err := exec.LookPath("portless")
	return err == nil
}

// ProxyPort returns the portless proxy port (default 1355).
func ProxyPort() int {
	out, err := exec.Command("portless", "list").CombinedOutput()
	if err != nil {
		return 1355
	}
	// Parse from the list output which contains URLs like "http://x.localhost:1355"
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.Index(line, ".localhost:"); idx >= 0 {
			rest := line[idx+len(".localhost:"):]
			// Extract port number before any whitespace
			portStr := strings.Fields(rest)[0]
			if p, err := strconv.Atoi(portStr); err == nil {
				return p
			}
		}
	}
	return 1355
}

// Alias registers a static route: name.localhost:<proxyPort> -> localhost:<hostPort>.
func Alias(name string, hostPort int) error {
	return exec.Command("portless", "alias", name, strconv.Itoa(hostPort), "--force").Run()
}

// Unalias removes a static route.
func Unalias(name string) error {
	return exec.Command("portless", "alias", "--remove", name).Run()
}
