package ports

import (
	"errors"
	"fmt"
	"sort"
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

// Watch polls the listener table every interval and reports opened and closed
// ports. A failed poll is passed to onError and skipped, so one transient
// netstat/ss/lsof failure does not end the watch. Only the first poll is fatal.
func Watch(interval time.Duration, onChange func(added, removed []Entry), onError func(error)) error {
	if interval <= 0 {
		return fmt.Errorf("watch interval must be positive, got %s", interval)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	return watchWith(ticker.C, List, onChange, onError)
}

func watchWith(ticks <-chan time.Time, list func() ([]Entry, error), onChange func(added, removed []Entry), onError func(error)) error {
	previousEntries, err := list()
	if err != nil {
		return err
	}
	previous := SnapshotOf(previousEntries)

	for range ticks {
		currentEntries, err := list()
		if err != nil {
			if onError != nil {
				onError(err)
			}
			continue
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
		sort.Slice(added, func(i, j int) bool { return added[i].Port < added[j].Port })
		sort.Slice(removed, func(i, j int) bool { return removed[i].Port < removed[j].Port })

		if len(added) > 0 || len(removed) > 0 {
			onChange(added, removed)
		}
		previous = current
	}
	return nil
}

// tolerateNoMatches treats exit status 1 as an empty result. lsof exits 1
// when no open file matches its filters, which here simply means no TCP
// listeners; any other failure is still returned.
func tolerateNoMatches(out []byte, err error) ([]byte, error) {
	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return out, nil
	}
	return out, err
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
