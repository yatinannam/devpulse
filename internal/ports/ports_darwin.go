//go:build darwin

package ports

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// List uses macOS lsof to enumerate TCP listeners. Process metadata may be
// unavailable when the caller lacks permission to inspect a process.
func List() ([]Entry, error) {
	out, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pcn").Output()
	if err != nil {
		return nil, fmt.Errorf("lsof: %w", err)
	}
	return parseLsof(string(out)), nil
}

func parseLsof(output string) []Entry {
	var entries []Entry
	var process, pid string
	for _, line := range strings.Split(output, "\\n") {
		if line == "" { continue }
		switch line[0] {
		case 'p': pid = strings.TrimPrefix(line, "p")
		case 'c': process = strings.TrimPrefix(line, "c")
		case 'n':
			addr := strings.TrimPrefix(line, "n")
			i := strings.LastIndex(addr, ":")
			if i < 0 { continue }
			port, err := strconv.Atoi(addr[i+1:]); if err != nil { continue }
			entries = append(entries, Entry{Port: port, Process: process, PID: pid, State: "LISTEN"})
		}
	}
	return entries
}

var _ = bufio.ErrInvalidUnreadByte
