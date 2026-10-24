package skills

import "testing"

func TestParseRequiredBins(t *testing.T) {
	content := "---\nname: demo\ndescription: x\nmetadata:\n  requires:\n    bins: [\"wecom-cli\", other-tool]\n---\n\nbody\n"
	bins := parseRequiredBins(content)
	if len(bins) != 2 || bins[0] != "wecom-cli" || bins[1] != "other-tool" {
		t.Fatalf("bins = %#v", bins)
	}
	if got := parseRequiredBins("no frontmatter"); got != nil {
		t.Fatalf("bins = %#v, want nil", got)
	}
}
