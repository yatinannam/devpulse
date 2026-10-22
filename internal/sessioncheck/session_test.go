package sessioncheck

import (
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestValidateSessionMetadata(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name string
		session traffic.Session
		wantValid bool
		wantCode string
	}{
		{name: "valid empty session", session: traffic.Session{Version: 1, CreatedAt: now}, wantValid: true},
		{name: "unsupported version", session: traffic.Session{Version: 2, CreatedAt: now}, wantCode: "unsupported_version"},
		{name: "missing creation time", session: traffic.Session{Version: 1}, wantCode: "missing_created_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Validate(tt.session)
			if got.Valid != tt.wantValid {
				t.Fatalf("Valid = %v, want %v; issues=%+v", got.Valid, tt.wantValid, got.Issues)
			}
			if tt.wantCode != "" && (len(got.Issues) == 0 || got.Issues[0].Code != tt.wantCode) {
				t.Fatalf("issues = %+v, want first code %q", got.Issues, tt.wantCode)
			}
		})
	}
}
