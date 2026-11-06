package ports

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

// Key identifies one process listening on one port. Two processes can share
// a port (127.0.0.1:8080 and [::1]:8080, or a forked server), so the port
// alone is not enough.
type Key struct {
	Port int
	PID  string
}

type Snapshot map[Key]Entry

func SnapshotOf(entries []Entry) Snapshot {
	s := make(Snapshot, len(entries))
	for _, e := range entries {
		s[Key{e.Port, e.PID}] = e
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

		// A restart within one tick (nodemon, air) keeps the port but
		// changes the PID, so it shows up as the old process closed and the
		// new one opened. Another process on the same port is untouched.
		var added, removed []Entry
		for key, entry := range current {
			if _, ok := previous[key]; !ok {
				added = append(added, entry)
			}
		}
		for key, entry := range previous {
			if _, ok := current[key]; !ok {
				removed = append(removed, entry)
			}
		}
		sortEntries(added)
		sortEntries(removed)

		if len(added) > 0 || len(removed) > 0 {
			onChange(added, removed)
		}
		previous = current
	}
}

// sortEntries orders entries by port, then PID, so the output of one poll
// does not depend on map iteration order.
func sortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Port != entries[j].Port {
			return entries[i].Port < entries[j].Port
		}
		return entries[i].PID < entries[j].PID
	})
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
