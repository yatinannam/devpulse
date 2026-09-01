package ports

import (
	"fmt"
	"time"
)

type Snapshot map[int]Entry

func SnapshotOf(entries []Entry) Snapshot {
	s := make(Snapshot, len(entries))
	for _, e := range entries {
		s[e.Port] = e
	}
	return s
}

func Watch(interval time.Duration, onChange func(added, removed []Entry)) error {
	previousEntries, err := List()
	if err != nil {
		return err
	}
	previous := SnapshotOf(previousEntries)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		currentEntries, err := List()
		if err != nil {
			return err
		}
		current := SnapshotOf(currentEntries)

		var added, removed []Entry
		for port, entry := range current {
			if _, ok := previous[port]; !ok {
				added = append(added, entry)
			}
		}
		for port, entry := range previous {
			if _, ok := current[port]; !ok {
				removed = append(removed, entry)
			}
		}

		if len(added) > 0 || len(removed) > 0 {
			onChange(added, removed)
		}
		previous = current
	}
}

func PrintChange(added, removed []Entry) {
	for _, e := range added {
		process := e.Process
		if process == "" {
			process = "-"
		}
		fmt.Printf("%s  + %-6d %-20s %-8s ● %s\n", time.Now().Format("15:04:05"), e.Port, process, e.PID, e.State)
	}
	for _, e := range removed {
		process := e.Process
		if process == "" {
			process = "-"
		}
		fmt.Printf("%s  - %-6d %-20s %-8s ○ CLOSED\n", time.Now().Format("15:04:05"), e.Port, process, e.PID)
	}
}
