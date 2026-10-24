package insights_test

import (
	"testing"

	"larder/internal/core/insights"
	"larder/internal/core/model"
)

func TestOverlapsFindsSimilarDescriptions(t *testing.T) {
	result := insights.Overlaps([]model.Skill{
		{ID: "a", Name: "diagnose", Description: "Diagnose hard bugs and performance regressions with a structured loop"},
		{ID: "b", Name: "diagnosing-bugs", Description: "Diagnose hard bugs and performance regressions using a structured loop"},
		{ID: "c", Name: "office", Description: "Create and edit PDF documents"},
	}, 5)
	if len(result) != 1 || result[0].A.Name == result[0].B.Name {
		t.Fatalf("overlaps = %#v", result)
	}
}
