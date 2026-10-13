package traffic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRequestJSONUsesStableFieldNames(t *testing.T) {
	request := Request{
		Method: "GET", Path: "/api/items/7", Status: 200,
		TargetHost: "localhost", TargetPort: 3000,
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	text := string(data)
	for _, key := range []string{`"time"`, `"method"`, `"path"`, `"status"`, `"latency"`, `"bytes"`, `"targetHost"`, `"targetPort"`} {
		if !strings.Contains(text, key) {
			t.Errorf("JSON %s does not contain %s", text, key)
		}
	}
	for _, key := range []string{`"TargetHost"`, `"TargetPort"`} {
		if strings.Contains(text, key) {
			t.Errorf("JSON contains unstable field %s: %s", key, text)
		}
	}
}

func TestLoadSessionAcceptsLegacyCapitalizedKeys(t *testing.T) {
	const legacy = `{
		"Version": 1,
		"CreatedAt": "2026-10-01T12:00:00Z",
		"Requests": [{
			"Time": "2026-10-01T12:00:00Z",
			"Method": "GET",
			"Path": "/legacy",
			"Status": 200,
			"Latency": 1000000,
			"Bytes": 12,
			"TargetHost": "localhost",
			"TargetPort": 3000
		}]
	}`
	var session Session
	if err := json.Unmarshal([]byte(legacy), &session); err != nil {
		t.Fatalf("decode legacy session: %v", err)
	}
	if session.Version != 1 || len(session.Requests) != 1 {
		t.Fatalf("unexpected legacy session: %+v", session)
	}
	got := session.Requests[0]
	if got.Path != "/legacy" || got.TargetHost != "localhost" || got.TargetPort != 3000 {
		t.Fatalf("legacy request fields did not round-trip: %+v", got)
	}
}
