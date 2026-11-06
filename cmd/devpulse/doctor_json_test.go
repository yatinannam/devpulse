package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/doctor"
	"github.com/yatinannam/devpulse/internal/traffic"
)

// These tests encode the envelope type doctorCommand actually prints, so a
// renamed or dropped key in main.go fails here.

func TestDoctorJSONEnvelopeUsesStableKeys(t *testing.T) {
	payload := newDoctorEnvelope("session.json", doctor.ProfileStrict, 4,
		[]doctor.Finding{{Level: "WARN", Title: "Slow requests", Detail: "slow", Advice: "inspect"}})
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal doctor envelope: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("decode doctor envelope: %v", err)
	}
	for _, key := range []string{"session", "profile", "requests", "findings"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("JSON envelope missing %q: %s", key, data)
		}
	}
	if len(fields) != 4 {
		t.Errorf("JSON envelope has unexpected keys: %s", data)
	}
}

func TestDoctorJSONEnvelopeEncodesNoFindingsAsEmptyArray(t *testing.T) {
	clean := []traffic.Request{{Time: time.Now(), Method: "GET", Path: "/ok", Status: 200}}
	payload := newDoctorEnvelope("session.json", doctor.ProfileDefault, len(clean),
		doctor.AnalyzeWithProfile(clean, doctor.ProfileDefault))
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"findings":[]`) {
		t.Fatalf("clean session should encode findings as [], got %s", data)
	}
}
