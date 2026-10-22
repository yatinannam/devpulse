package sessioncheck

import (
	"testing"
	"time"
)

func TestValidateNumericRequestFields(t *testing.T) {
	req := validRequest()
	req.Status = 99
	req.Latency = -time.Millisecond
	req.TargetPort = 65536
	req.TargetHost = ""
	got := Validate(validSession(req))
	codes := map[string]bool{}
	for _, issue := range got.Issues { codes[issue.Code] = true }
	for _, code := range []string{"invalid_status", "negative_latency", "invalid_target_port", "missing_target_host"} {
		if !codes[code] { t.Errorf("missing issue %q in %+v", code, got.Issues) }
	}
}

func TestValidateIssueOrderAndRequestIndicesAreStable(t *testing.T) {
	first := validRequest()
	first.Time = time.Time{}
	first.Method = ""
	first.Path = ""
	first.Status = 600
	first.Latency = -1
	first.TargetPort = 0
	second := validRequest()
	second.Status = 600
	got := Validate(validSession(first, second))
	seenFirstTimestamp, seenSecondStatus := false, false
	for _, issue := range got.Issues {
		if issue.Code == "missing_timestamp" && issue.Request == 1 { seenFirstTimestamp = true }
		if issue.Code == "invalid_status" && issue.Request == 2 { seenSecondStatus = true }
	}
	if !seenFirstTimestamp || !seenSecondStatus { t.Fatalf("request indices not one-based: %+v", got.Issues) }
}
