// Package insights contains local, read-only analysis of the scanned inventory.
package insights

import (
	"sort"
	"strings"
	"unicode"

	"larder/internal/core/model"
)

type Overlap struct {
	A     model.Skill
	B     model.Skill
	Score float64
}

// Overlaps finds likely duplicate Skill descriptions. It is intentionally
// local-only and deterministic; no descriptions leave the machine.
func Overlaps(skills []model.Skill, limit int) []Overlap {
	if limit <= 0 {
		limit = 10
	}
	type candidate struct {
		skill  model.Skill
		tokens map[string]bool
	}
	seen := map[string]bool{}
	var candidates []candidate
	for _, skill := range skills {
		if skill.IsAgentLocal || skill.Description == "" || seen[skill.Name] {
			continue
		}
		seen[skill.Name] = true
		candidates = append(candidates, candidate{skill: skill, tokens: tokenize(skill.Description)})
	}
	var result []Overlap
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			score := jaccard(candidates[i].tokens, candidates[j].tokens)
			if score < 0.55 {
				continue
			}
			result = append(result, Overlap{A: candidates[i].skill, B: candidates[j].skill, Score: score})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func tokenize(value string) map[string]bool {
	value = strings.ToLower(value)
	tokens := map[string]bool{}
	var latin strings.Builder
	flush := func() {
		if latin.Len() >= 3 {
			tokens[latin.String()] = true
		}
		latin.Reset()
	}
	var cjk []rune
	addCJK := func() {
		for i := 0; i+1 < len(cjk); i++ {
			tokens[string(cjk[i:i+2])] = true
		}
		cjk = cjk[:0]
	}
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			flush()
			cjk = append(cjk, r)
			continue
		}
		if len(cjk) > 0 {
			addCJK()
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			latin.WriteRune(r)
			continue
		}
		flush()
	}
	addCJK()
	flush()
	return tokens
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for key := range a {
		if b[key] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}
