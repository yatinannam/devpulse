package traffic

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateKeepsMultiByteRunesWhole(t *testing.T) {
	path := "/api/" + strings.Repeat("é", 40) // 45 runes, 85 bytes

	got := truncate(path, 32)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate split a rune: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 32 {
		t.Fatalf("truncate returned %d runes, want 32: %q", n, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated path %q does not end with an ellipsis", got)
	}
}

func TestTruncateCountsRunesNotBytes(t *testing.T) {
	for _, tc := range []struct {
		in   string
		n    int
		want string
	}{
		{in: "/café", n: 5, want: "/café"},
		{in: "/api/users/123456", n: 8, want: "/api/us…"},
		{in: "/日本語/パス", n: 4, want: "/日本…"},
	} {
		if got := truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}
