//go:build linux

package ports

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// processInfo matches the users:(("name",pid=1234,fd=6)) suffix that
// `ss -ltnp` appends to each listening socket line.
var processInfo = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

func List() ([]Entry, error) {
	out, err := exec.Command("ss", "-ltnp").Output()
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	return parseSS(string(out)), nil
}

// parseSS parses the output of `ss -ltnp` into port Entries, including
// process name and PID when the caller has permission to see them
// (root, or the process' own owner).
func parseSS(output string) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "LISTEN" {
			continue
		}
		addr := fields[3]
		i := strings.LastIndex(addr, ":")
		if i < 0 {
			continue
		}
		p, err := strconv.Atoi(strings.Trim(addr[i+1:], "]"))
		if err != nil {
			continue
		}
		entry := Entry{Port: p, State: "LISTEN"}
		if m := processInfo.FindStringSubmatch(line); m != nil {
			entry.Process = m[1]
			entry.PID = m[2]
		}
		entries = append(entries, entry)
	}
	return entries
}
