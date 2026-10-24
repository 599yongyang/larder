package usage_test

import (
	"testing"

	"larder/internal/core/usage"
)

func TestParseZshHistoryCountsKnownCommands(t *testing.T) {
	result := usage.ParseZshHistory([]byte(": 1:0;git status\n: 2:0;jq . file\nls -la\njq . other\n"), map[string]bool{"git": true, "jq": true}, 10)
	if len(result) != 2 || result[0].Name != "jq" || result[0].Count != 2 {
		t.Fatalf("stats = %#v", result)
	}
}
