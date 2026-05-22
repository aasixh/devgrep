package parser

import (
	"strings"
	"testing"
)

func TestDetectSeverity(t *testing.T) {
	cases := map[string]string{
		"INFO server started":             SeverityInfo,
		"[WARN] retrying connection":      SeverityWarn,
		"ERROR failed to connect":         SeverityError,
		"DEBUG cache miss":                SeverityDebug,
		"trace without severity":          "",
		"2026-01-01 warning disk is full": SeverityWarn,
	}
	for line, want := range cases {
		if got := DetectSeverity(line); got != want {
			t.Fatalf("DetectSeverity(%q) = %q, want %q", line, got, want)
		}
	}
}

func TestErrorGroupKeyNormalizesNumbers(t *testing.T) {
	a := ErrorGroupKey("ERROR request 123 failed at /tmp/run-456.log")
	b := ErrorGroupKey("ERROR request 999 failed at /tmp/run-111.log")
	if a != b {
		t.Fatalf("group keys differ: %q != %q", a, b)
	}
}

func TestParseMarkdownFragments(t *testing.T) {
	fragments, err := ParseMarkdownFragments(strings.NewReader("# Deploy\nRun make release.\n\nCheck logs after deploy.\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 3 {
		t.Fatalf("fragments = %#v", fragments)
	}
	if fragments[0].Content != "Deploy" {
		t.Fatalf("first fragment = %q", fragments[0].Content)
	}
}
