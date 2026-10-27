package clis

import "testing"

func TestParseVersionOutput(t *testing.T) {
	for input, want := range map[string]string{
		"codex-cli 0.164.0": "0.164.0",
		"2.1.0":             "2.1.0",
		"v1.2.3\n":          "1.2.3",
		"no version":         "",
	} {
		if got := ParseVersionOutput(input); got != want {
			t.Fatalf("ParseVersionOutput(%q) = %q, want %q", input, got, want)
		}
	}
}
