package catalog

import "testing"

func TestLocalPopularContainsDesignTasteSource(t *testing.T) {
	for _, skill := range LocalPopular() {
		if skill.Name == "design-taste-frontend" {
			if skill.Source != "leonxlnx/taste-skill" {
				t.Fatalf("source = %q", skill.Source)
			}
			return
		}
	}
	t.Fatal("design-taste-frontend missing from catalog snapshot")
}
