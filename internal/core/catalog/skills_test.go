package catalog

import (
	"encoding/json"
	"testing"
)

func TestExtractInitialSkillsFromRSC(t *testing.T) {
	body := []byte(`<script>self.__next_f.push([1,"4e:[\"$\",\"$L55\",null,{\"initialSkills\":[{\"source\":\"vercel-labs/skills\",\"skillId\":\"find-skills\",\"name\":\"find-skills\",\"installs\":4011690,\"installsYesterday\":4000000,\"change\":11690}]}]\n"])</script>`)
	skills, err := extractInitialSkills(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].ID != "vercel-labs/skills/find-skills" || skills[0].Change != 11690 {
		t.Fatalf("unexpected skills: %+v", skills)
	}
}

func TestFrontmatterDescription(t *testing.T) {
	markdown := "---\nname: pdf\ndescription: Use this skill when the user wants to work with PDF files.\n---\n\n# PDF\n"
	if got := frontmatterDescription(markdown); got != "Use this skill when the user wants to work with PDF files." {
		t.Fatalf("description = %q", got)
	}
	block := "---\ndescription: |\n  First line\n  second line\nlicense: MIT\n---\n"
	if got := frontmatterDescription(block); got != "First line second line" {
		t.Fatalf("block description = %q", got)
	}
}

func TestFirstParagraphFallback(t *testing.T) {
	markdown := "---\nname: pdf\n---\n\n# PDF\n\nUse this skill for PDF files.\nSecond line.\n"
	if got := frontmatterDescription(markdown); got != "Use this skill for PDF files. Second line." {
		t.Fatalf("fallback = %q", got)
	}
}

func TestPopularSnapshotIsValid(t *testing.T) {
	var skills []Skill
	if err := json.Unmarshal(popularSnapshot, &skills); err != nil {
		t.Fatal(err)
	}
	tidy := tidyPopular(skills)
	if len(tidy) == 0 || len(tidy) > 60 {
		t.Fatalf("popular snapshot = %d", len(tidy))
	}
}
