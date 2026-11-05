//go:build darwin

package ports

import (
	"fmt"
	"os/exec"
)

// List uses macOS lsof to enumerate TCP listeners. Process metadata may be
// unavailable when the caller lacks permission to inspect a process.
// The output is parsed by parseLsof in lsof.go, which builds on every OS.
func List() ([]Entry, error) {
	out, err := tolerateNoMatches(exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcn").Output())
	if err != nil {
		return nil, fmt.Errorf("lsof: %w", err)
	}
	return parseLsof(string(out)), nil
}
