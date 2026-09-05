//go:build windows

package ports

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestParseTasklistCSV(t *testing.T) {
	const sample = `"node.exe","1234","Console","1","50,000 K"
"postgres.exe","5678","Services","0","120,000 K"
"Code.exe","9999","Console","1","200,000 K"
`

	m := make(map[string]string)
	reader := csv.NewReader(strings.NewReader(sample))
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) >= 2 {
			pid := strings.TrimSpace(record[1])
			name := strings.TrimSpace(record[0])
			name = strings.TrimSuffix(name, ".exe")
			m[pid] = name
		}
	}

	if m["1234"] != "node" {
		t.Errorf("pid 1234: got %q, want node", m["1234"])
	}
	if m["5678"] != "postgres" {
		t.Errorf("pid 5678: got %q, want postgres", m["5678"])
	}
	if m["9999"] != "Code" {
		t.Errorf("pid 9999: got %q, want Code", m["9999"])
	}
}