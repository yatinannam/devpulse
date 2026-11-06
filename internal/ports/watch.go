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

// Watch polls the listener table every interval and reports ports opened and
// closed since initial, the listing the caller already showed. Starting from
// that listing instead of polling again means nothing that changes between
// the two is missed. A failed poll is passed to onError and skipped, so one
// transient netstat/ss/lsof failure does not end the watch.
func Watch(initial []Entry, interval time.Duration, onChange func(added, removed []Entry), onError func(error)) error {
	if interval <= 0 {
		return fmt.Errorf("watch interval must be positive, got %s", interval)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	watchWith(initial, ticker.C, List, onChange, onError)
	return nil
}

func watchWith(initial []Entry, ticks <-chan time.Time, list func() ([]Entry, error), onChange func(added, removed []Entry), onError func(error)) {
	previous := SnapshotOf(initial)

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
			old, ok := previous[port]
			if !ok {
				added = append(added, entry)
			} else if old.PID != entry.PID {
				// A restart within one tick (nodemon, air) keeps the port
				// but changes the PID: report the old process as closed and
				// the new one as opened.
				removed = append(removed, old)
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

// PrintChange writes opened and closed ports to stdout; see RenderChange.
func PrintChange(added, removed []Entry) {
	printChange(added, removed)
}
