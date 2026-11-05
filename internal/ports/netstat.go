package ports

import (
	"bufio"
	"encoding/csv"
	"strconv"
	"strings"
)

// parseNetstat extracts TCP listeners from `netstat -ano` output. Windows
// localizes the State column (LISTENING is ABHÖREN on German systems), so a
// listener is recognised by its wildcard foreign address instead. IPv4 and
// IPv6 rows for the same port and PID are collapsed into one entry.
func parseNetstat(out string) []Entry {
	type key struct {
		port int
		pid  string
	}
	seen := map[key]bool{}
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if foreign := fields[2]; foreign != "0.0.0.0:0" && foreign != "[::]:0" {
			continue
		}
		local := fields[1]
		i := strings.LastIndex(local, ":")
		if i < 0 {
			continue
		}
		port, err := strconv.Atoi(local[i+1:])
		if err != nil {
			continue
		}
		pid := fields[len(fields)-1]
		k := key{port, pid}
		if seen[k] {
			continue
		}
		seen[k] = true
		entries = append(entries, Entry{Port: port, PID: pid, State: "LISTENING"})
	}
	return entries
}

// parseTasklist maps PIDs to process names from `tasklist /FO CSV /NH`.
func parseTasklist(out string) map[string]string {
	m := make(map[string]string)
	reader := csv.NewReader(strings.NewReader(out))
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) >= 2 {
			pid := strings.TrimSpace(record[1])
			name := strings.TrimSuffix(strings.TrimSpace(record[0]), ".exe")
			m[pid] = name
		}
	}
	return m
}
