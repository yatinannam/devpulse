package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteJSONDoesNotEscapeHTML(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, map[string]string{"path": "/search?a=1&b=<x>"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, `"/search?a=1&b=<x>"`) {
		t.Fatalf("path was escaped: %s", got)
	}
	// Every HTML escape the encoder could emit starts with this sequence.
	if strings.Contains(got, `\u00`) {
		t.Errorf("output contains an HTML escape: %s", got)
	}
}

func TestWriteJSONIndentsAndEndsWithNewline(t *testing.T) {
	var out bytes.Buffer
	if err := WriteJSON(&out, map[string]int{"total": 1}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "{\n  \"total\": 1\n}\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWriteJSONReportsEncodeErrors(t *testing.T) {
	if err := WriteJSON(&bytes.Buffer{}, make(chan int)); err == nil {
		t.Fatal("expected an unsupported value to fail")
	}
}
