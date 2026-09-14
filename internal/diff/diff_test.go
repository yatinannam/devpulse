package diff

import (
    "testing"
    "time"

    "github.com/yatinannam/devpulse/internal/baseline"
)

func TestCompareFindsServiceAndTrafficChanges(t *testing.T) {
    base := baseline.Snapshot{
        Services: []baseline.Service{{Port: 3000, Kind: "node"}},
        Requests: baseline.Traffic{Total: 10, Errors: 1, Average: int64(20 * time.Millisecond)},
    }
    current := baseline.Snapshot{
        Services: []baseline.Service{{Port: 4000, Kind: "go"}},
        Requests: baseline.Traffic{Total: 14, Errors: 3, Average: int64(50 * time.Millisecond)},
    }
    changes := Compare(base, current)
    if len(changes) != 3 {
        t.Fatalf("got %d changes, want 3: %#v", len(changes), changes)
    }
    kinds := map[Kind]bool{}
    for _, c := range changes { kinds[c.Kind] = true }
    if !kinds[ServiceAdded] || !kinds[ServiceRemoved] || !kinds[TrafficChanged] {
        t.Fatalf("missing expected change kinds: %#v", changes)
    }
}

func TestCompareIgnoresIdenticalSnapshots(t *testing.T) {
    snapshot := baseline.Snapshot{Requests: baseline.Traffic{Total: 2}}
    if got := Compare(snapshot, snapshot); got != nil {
        t.Fatalf("got changes for identical snapshots: %#v", got)
    }
}
