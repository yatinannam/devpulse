//go:build linux

package ports

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func List() ([]Entry, error) {
	out, err := exec.Command("ss", "-ltnp").Output()
	if err != nil {
		return nil, fmt.Errorf("ss: %w", err)
	}
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
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
		entries = append(entries, Entry{Port: p, State: "LISTEN"})
	}
	return entries, nil
}
