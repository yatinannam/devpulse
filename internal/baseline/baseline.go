package baseline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/yatinannam/devpulse/internal/discovery"
	"github.com/yatinannam/devpulse/internal/status"
	"github.com/yatinannam/devpulse/internal/traffic"
)

const Version = 1
const FileName = "baseline.json"

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type Snapshot struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Services  []Service `json:"services"`
	Requests  Traffic   `json:"traffic"`
}

type Service struct {
	Port    int    `json:"port"`
	Kind    string `json:"kind"`
	HTTP    bool   `json:"http"`
	Process string `json:"process,omitempty"`
}

type Traffic struct {
	Total     int        `json:"total"`
	Errors    int        `json:"errors"`
	Slow      int        `json:"slow"`
	Average   int64      `json:"average_ns"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Count   int    `json:"count"`
	Errors  int    `json:"errors"`
	Average int64  `json:"average_ns"`
}

func Build(services []discovery.Service, requests []traffic.Request) Snapshot {
	summary := status.Build(requests)
	endpoints := status.Endpoints(requests)
	out := make([]Endpoint, 0, len(endpoints))
	for _, e := range endpoints {
		out = append(out, Endpoint{
			Method:  e.Method,
			Path:    e.Path,
			Count:   e.Count,
			Errors:  e.Errors,
			Average: int64(e.Average),
		})
	}
	svcs := make([]Service, 0, len(services))
	for _, s := range services {
		svcs = append(svcs, Service{
			Port:    s.Port,
			Kind:    s.Kind,
			HTTP:    s.HTTP,
			Process: s.Process,
		})
	}
	return Snapshot{
		Version:   Version,
		CreatedAt: time.Now(),
		Services:  svcs,
		Requests: Traffic{
			Total:     summary.Total,
			Errors:    summary.Errors,
			Slow:      summary.Slow,
			Average:   int64(summary.Average),
			Endpoints: out,
		},
	}
}

func Path(projectDir string) string {
	return filepath.Join(projectDir, ".devpulse", FileName)
}

func Save(path string, snapshot Snapshot) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create baseline directory: %w", err)
	}
	// Write a uniquely named temporary file in the target directory, flush
	// it, then rename it into place, so a crash never leaves a partial
	// baseline and a failed rename leaves nothing behind.
	tmp, err := os.CreateTemp(dir, ".baseline-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary baseline: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary baseline: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write baseline: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush baseline: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary baseline: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("save baseline: %w", err)
	}
	return nil
}

func Load(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read baseline: %w", err)
	}
	var snapshot Snapshot
	// A file re-saved by Windows PowerShell 5.1 starts with a UTF-8 byte
	// order mark, which encoding/json rejects; sessions already ignore it.
	if err := json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode baseline: %w", err)
	}
	if snapshot.Version != Version {
		return Snapshot{}, fmt.Errorf("unsupported baseline version %d", snapshot.Version)
	}
	return snapshot, nil
}
