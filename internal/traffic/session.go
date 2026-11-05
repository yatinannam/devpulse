package traffic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	Requests  []Request `json:"requests"`
}

func SaveSession(path string, entries []Request) error {
	s := Session{Version: 1, CreatedAt: time.Now(), Requests: entries}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary session: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary session: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write session: %w", err)
	}
	// Flush to disk before the rename, so a crash or power loss cannot leave
	// an empty or partial file under the session's name.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush session: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary session: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

// utf8BOM is the byte order mark Windows PowerShell 5.1 writes at the start
// of UTF-8 files. encoding/json rejects it as an invalid character.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// DecodeSession decodes a session file's contents, ignoring a leading UTF-8
// byte order mark. It does not check the version, so validators can report
// an unsupported version as a finding; it returns the JSON error unwrapped.
func DecodeSession(data []byte) (Session, error) {
	var s Session
	if err := json.Unmarshal(bytes.TrimPrefix(data, utf8BOM), &s); err != nil {
		return Session{}, err
	}
	return s, nil
}

func LoadSession(path string) (Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	s, err := DecodeSession(data)
	if err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	if s.Version != 1 {
		return Session{}, fmt.Errorf("unsupported session version %d", s.Version)
	}
	return s, nil
}
