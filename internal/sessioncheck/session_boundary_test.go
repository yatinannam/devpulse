package sessioncheck

import "testing"

func TestValidateAcceptsHTTPAndPortBoundaries(t *testing.T) {
	req := validRequest()
	req.Status = 100
	req.TargetPort = 1
	if got := Validate(validSession(req)); !got.Valid {
		t.Fatalf("lower valid boundaries rejected: %+v", got.Issues)
	}
	req.Status = 599
	req.TargetPort = 65535
	if got := Validate(validSession(req)); !got.Valid {
		t.Fatalf("upper valid boundaries rejected: %+v", got.Issues)
	}
}
