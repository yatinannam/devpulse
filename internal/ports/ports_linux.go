//go:build linux

package ports

import (
	"fmt"
	"os/exec"
)

// List enumerates TCP listeners with `ss -ltnp`. The output is parsed by
// parseSS in ss.go, which builds on every OS so its tests run everywhere.
func List() ([]Entry, error) {
	out, err := exec.Command("ss", "-ltnp").Output()
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	return parseSS(string(out)), nil
}
