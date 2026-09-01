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

	for i := range entries {
		if name, err := processName(entries[i].PID); err == nil {
			entries[i].Process = name
		}
	}
	return entries, nil
}

func processName(pid string) (string, error) {
	out, err := exec.Command(
		"tasklist",
		"/FI", "PID eq "+pid,
		"/FO", "CSV",
		"/NH",
	).Output()
	if err != nil {
		return "", err
	}

	reader := csv.NewReader(strings.NewReader(string(out)))
	record, err := reader.Read()
	if err != nil || len(record) == 0 || strings.HasPrefix(record[0], "INFO:") {
		return "", fmt.Errorf("process not found")
	}
	return record[0], nil
}
