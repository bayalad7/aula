//go:build linux && admintest

package policy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// resolveAsStudent resolves as an ordinary user (nobody), through the
// system's resolver: root's queries skip Aula's table on purpose.
func resolveAsStudent(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "setpriv", "--reuid=65534", "--regid=65534", "--clear-groups",
		"getent", "ahosts", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("getent %s: %w %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// aulaRules is Aula's nftables table ("" if there is none).
func aulaRules(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("nft", "list", "table", "inet", "aula").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
