// Package usage parses local shell history only when the user opts in.
package usage

import (
	"sort"
	"strings"

	"larder/internal/core/model"
)

// ParseZshHistory counts first command tokens that match known CLI names.
// Extended zsh lines (`: timestamp:duration;command`) and plain lines are
// supported. The original history is never written.
func ParseZshHistory(data []byte, known map[string]bool, limit int) []model.UsageStat {
	counts := map[string]int{}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ": ") {
			if idx := strings.Index(line, ";"); idx >= 0 {
				line = strings.TrimSpace(line[idx+1:])
			}
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if strings.Contains(name, "=") {
			continue
		}
		name = strings.TrimPrefix(name, "command ")
		if !known[name] && !known[strings.TrimSuffix(name, ".exe")] {
			continue
		}
		counts[name]++
	}
	result := make([]model.UsageStat, 0, len(counts))
	for name, count := range counts {
		result = append(result, model.UsageStat{Name: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}
