//go:build windows

package ports

import (
	"fmt"
	"os/exec"
)

// List enumerates IPv4 and IPv6 TCP listeners. Without -p, netstat prints
// both the TCP and TCPv6 tables; `-p TCP` would hide dual-stack servers such
// as Node on [::] and Vite on [::1].
func List() ([]Entry, error) {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w", err)
	}

	entries := parseNetstat(string(out))
	pMap := processMap()
	for i := range entries {
		if name, ok := pMap[entries[i].PID]; ok {
			entries[i].Process = name
		}
	}
	return entries, nil
}

func processMap() map[string]string {
	out, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil
	}
	return parseTasklist(string(out))
}
