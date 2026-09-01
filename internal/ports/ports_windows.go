//go:build windows

package ports

import (
	"bufio"
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

	line := strings.TrimSpace(string(out))
	if line == "" || strings.HasPrefix(line, "INFO:") {
		return "", fmt.Errorf("process not found")
	}

	fields := strings.Split(line, "","")
	if len(fields) == 0 {
		return "", fmt.Errorf("process not found")
	}

	return strings.Trim(fields[0], """), nil
}
