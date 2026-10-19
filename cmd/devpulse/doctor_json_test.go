package main

import (
	"encoding/json"
	"testing"

	"github.com/yatinannam/devpulse/internal/doctor"
)

func TestDoctorJSONEnvelopeUsesStableKeys(t *testing.T) {
	payload := struct {
		Session string `json:"session"`
		Profile doctor.Profile `json:"profile"`
		Requests int `json:"requests"`
		Findings []doctor.Finding `json:"findings"`
	}{
		Session: "session.json",
		Profile: doctor.ProfileStrict,
		Requests: 4,
		Findings: []doctor.Finding{{Level: "WARN", Title: "Slow requests", Detail: "slow", Advice: "inspect"}},
	}
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
}
