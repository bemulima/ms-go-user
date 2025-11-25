package e2e

import (
	"os"
	"os/exec"
	"testing"
)

// TestAvatarE2E runs the cross-service avatar flow using the bash helper script.
// Requires Docker/Compose, jq, and sibling service directories present.
func TestAvatarE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e in short mode")
	}
	cmd := exec.Command("bash", "./test/e2e/run-avatar-e2e.sh")
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("e2e script failed: %v\nOutput:\n%s", err, string(out))
	}
	t.Logf("e2e output:\n%s", string(out))
}
