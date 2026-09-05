//go:build windows

package ports

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func List() ([]Entry, error) {
	out, err := exec.Command("netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w", err)
	}

	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}

		port := fields[1]
		if i := strings.LastIndex(port, ":"); i >= 0 {
			port = port[i+1:]
		}
		p, err := strconv.Atoi(port)
		if err != nil {
			continue
		}

		entries = append(entries, Entry{
			Port:  p,
			PID:   fields[4],
			State: "LISTENING",
		})
	}

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

	m := make(map[string]string)
	reader := csv.NewReader(strings.NewReader(string(out)))
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
	return m
}
