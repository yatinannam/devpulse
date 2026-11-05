package ports

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// processInfo matches the users:(("name",pid=1234,fd=6)) suffix that
// `ss -ltnp` appends to each listening socket line.
var processInfo = regexp.MustCompile(`users:\(\("([^"]+)",pid=(\d+)`)

// parseSS parses the output of `ss -ltnp` into port Entries, including
// process name and PID when the caller has permission to see them
// (root, or the process' own owner). A process bound to both an IPv4 and an
// IPv6 address gets one row per address; rows with the same port and PID are
// collapsed into one entry, as parseNetstat does on Windows.
func parseSS(output string) []Entry {
	type key struct {
		port int
		pid  string
	}
	seen := map[key]bool{}
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
		k := key{entry.Port, entry.PID}
		if seen[k] {
			continue
		}
		seen[k] = true
		entries = append(entries, entry)
	}
	return entries
}
