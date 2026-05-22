package utils

import (
	"path/filepath"
	"testing"
)

func TestExpandPathWithHome(t *testing.T) {
	home := "/home/alice"
	cases := []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/work", filepath.Join(home, "work")},
		{"$HOME", home},
		{"$HOME/work/devgrep", filepath.Join(home, "work", "devgrep")},
		{"/tmp", "/tmp"},
	}
	for _, tc := range cases {
		got, err := ExpandPathWithHome(tc.in, home)
		if err != nil {
			t.Fatalf("ExpandPathWithHome(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ExpandPathWithHome(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestInvalidHistoryCWD(t *testing.T) {
	if !InvalidHistoryCWD("/sys/kernel/boot_params/legacy") {
		t.Fatal("expected kernel path to be invalid")
	}
	if InvalidHistoryCWD("/home/alice/work/devgrep") {
		t.Fatal("expected project path to be valid")
	}
}

func TestMalformedCWDPath(t *testing.T) {
	bad := []string{
		"/tmp/~/work/url-shortner",
		"/tmp/~/work/url-shortner/work/algo-lang/~/work/work/algo-lang",
		"/tmp/~work",
	}
	for _, path := range bad {
		if !MalformedCWDPath(path) {
			t.Fatalf("MalformedCWDPath(%q) = false, want true", path)
		}
	}
	good := []string{
		"/home/alice/work/devgrep",
		"",
	}
	for _, path := range good {
		if MalformedCWDPath(path) {
			t.Fatalf("MalformedCWDPath(%q) = true, want false", path)
		}
	}
}

func TestNormalizeCWDAndRelHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "work", "devgrep")
	norm, ok := NormalizeCWD(project, home)
	if !ok || norm != project {
		t.Fatalf("NormalizeCWD = (%q, %v)", norm, ok)
	}
	if got := RelHome(norm); got != "~/work/devgrep" {
		t.Fatalf("RelHome = %q", got)
	}
	if _, ok := NormalizeCWD("/tmp/~/work/devgrep", home); ok {
		t.Fatal("expected malformed cwd to be rejected")
	}
}

func TestSanitizeCWD(t *testing.T) {
	home := "/home/alice"
	if got := SanitizeCWD("/tmp/~/work", home, home); got != home {
		t.Fatalf("SanitizeCWD = %q, want %q", got, home)
	}
	want := filepath.Join(home, "work", "portfolio")
	if got := SanitizeCWD("~/work/portfolio", home, home); got != want {
		t.Fatalf("SanitizeCWD = %q, want %q", got, want)
	}
}
