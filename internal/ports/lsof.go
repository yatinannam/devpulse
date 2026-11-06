package ports

import (
	"strconv"
	"strings"
)

// parseLsof parses `lsof -nP -iTCP -sTCP:LISTEN -F pcn` field output. lsof
// reports the TCP4 and TCP6 descriptors of a dual-stack process separately;
// rows with the same port and PID are collapsed into one entry, as
// parseNetstat does on Windows.
func parseLsof(output string) []Entry {
	type key struct {
		port int
		pid  string
	}
	seen := map[key]bool{}
	var entries []Entry
	var process, pid string
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			pid = strings.TrimPrefix(line, "p")
		case 'c':
			process = strings.TrimPrefix(line, "c")
		case 'n':
			addr := strings.TrimPrefix(line, "n")
			i := strings.LastIndex(addr, ":")
			if i < 0 {
				continue
			}
			port, err := strconv.Atoi(addr[i+1:])
			if err != nil {
				continue
			}
			k := key{port, pid}
			if seen[k] {
				continue
			}
			seen[k] = true
			entries = append(entries, Entry{Port: port, Process: process, PID: pid, State: "LISTEN"})
		}
	}
	return entries
}
