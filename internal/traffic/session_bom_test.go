package traffic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// utf8BOMPrefix is what Windows PowerShell 5.1 writes at the start of a file
// saved with `Set-Content -Encoding UTF8` or `Out-File`.
const utf8BOMPrefix = "\xEF\xBB\xBF"

func writeSessionFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSessionAcceptsAUTF8BOM(t *testing.T) {
	path := writeSessionFile(t, utf8BOMPrefix+`{"version":1,"createdAt":"2026-01-02T03:04:05Z","requests":[{"method":"GET","path":"/bom"}]}`)

	s, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession with a UTF-8 BOM: %v", err)
	}
	if len(s.Requests) != 1 || s.Requests[0].Path != "/bom" {
		t.Fatalf("unexpected session: %+v", s)
	}
}

func TestLoadSessionWithABOMStillRejectsUnknownVersions(t *testing.T) {
	path := writeSessionFile(t, utf8BOMPrefix+`{"version":2,"requests":[]}`)

	_, err := LoadSession(path)
	if err == nil || !strings.Contains(err.Error(), "unsupported session version 2") {
		t.Fatalf("LoadSession error = %v, want the unsupported version error", err)
	}
}

func TestDecodeSessionStripsABOMWithoutCheckingTheVersion(t *testing.T) {
	s, err := DecodeSession([]byte(utf8BOMPrefix + `{"version":2,"requests":[{"path":"/v2"}]}`))
	if err != nil {
		t.Fatalf("DecodeSession: %v", err)
	}
	if s.Version != 2 || len(s.Requests) != 1 || s.Requests[0].Path != "/v2" {
		t.Fatalf("unexpected session: %+v", s)
	}
}

func TestDecodeSessionReportsMalformedJSON(t *testing.T) {
	if _, err := DecodeSession([]byte(utf8BOMPrefix + `{"version":`)); err == nil {
		t.Fatal("DecodeSession accepted truncated JSON")
	}
}
