package app

import (
	"context"
	"os/exec"
	"time"
)

// dockerAvailable returns true iff the test environment has a working docker
// socket. Used to skip integration tests when docker isn't reachable so that
// `go test ./...` stays green on CI runners without a docker daemon.
func dockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "version").Run(); err != nil {
		return false
	}
	// `docker version` exits 0 even when the client can't reach the daemon
	// — verify the daemon side too.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	return exec.CommandContext(ctx2, "docker", "info").Run() == nil
}
