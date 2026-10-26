// Package xray builds a read-only static resolution report for one Agent.
package xray

import (
	"errors"
	"sort"
	"strings"
	"time"

	"larder/internal/core/model"
)

const schemaVersion = "1"

type Input struct {
	Scan            model.ScanResult
	Instructions    []model.InstructionFile
	MCPServers      []model.MCPServer
	Settings        model.Settings
	AgentID         string
	Project         string
	CompareAgentIDs []string
}

func Build(in Input) (model.XrayReport, error) {
	root, ok := findRoot(in.Scan.Roots, in.AgentID)
	if !ok {
		return model.XrayReport{}, errors.New("Agent directory not found")
	}
	roots := comparisonRoots(in.Scan, in.Settings, root, in.CompareAgentIDs)
	groups := groupCandidates(in, root)
	entries := make([]model.XrayEntry, 0, len(groups))
	for _, candidates := range groups {
		entries = append(entries, resolveEntry(in, candidates))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Status != entries[j].Status {
			return entries[i].Status < entries[j].Status
		}
		return strings.ToLower(entries[i].Skill.Name) < strings.ToLower(entries[j].Skill.Name)
	})

	active := make([]model.XrayEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Status == model.XrayStatusActive || entry.Status == model.XrayStatusCandidate {
			active = append(active, entry)
		}
	}
	skillTokens := 0
	for _, entry := range active {
		skillTokens += entry.Skill.DescTokens
	}
	instructions := relevantInstructions(in)
	instructionTokens := 0
	for _, file := range instructions {
		instructionTokens += (int(file.Size) + 3) / 4
	}
	mcps := relevantMCPs(in, root)
	contributors := append([]model.XrayEntry(nil), active...)
	sort.SliceStable(contributors, func(i, j int) bool { return contributors[i].Skill.DescTokens > contributors[j].Skill.DescTokens })
	if len(contributors) > 10 {
		contributors = contributors[:10]
	}

	report := model.XrayReport{
		SchemaVersion:  schemaVersion,
		AgentID:        root.ID,
		AgentLabel:     root.Label,
		Project:        in.Project,
		Root:           root,
		Entries:        entries,
		ActiveCount:    len(active),
		MissingCount:   countStatus(entries, model.XrayStatusMissing),
		CandidateCount: countStatus(entries, model.XrayStatusCandidate),
		BrokenCount:    countStatus(entries, model.XrayStatusBroken),
		Budget: model.XrayBudget{
			SkillTokens:       skillTokens,
			InstructionTokens: instructionTokens,
			KnownTokens:       skillTokens + instructionTokens,
			Unmodeled:         true,
		},
		Instructions: instructions,
		MCPServers:   mcps,
		Contributors: contributors,
		Compare:      buildComparison(in, roots, entries),
		GeneratedAt:  time.Now().UTC(),
	}
	return report, nil
}

func findRoot(roots []model.Root, id string) (model.Root, bool) {
	for _, root := range roots {
		if root.ID == id {
			return root, true
		}
	}
	return model.Root{}, false
}

func ignoredRoot(settings model.Settings, root model.Root) bool {
	for _, ignored := range settings.IgnoredAgents {
		if ignored.Path == root.Path {
			return true
		}
	}
	return false
}

func comparisonRoots(scan model.ScanResult, settings model.Settings, current model.Root, requested []string) []model.Root {
	hasGlobal := false
	for _, skill := range scan.Skills {
		if skill.RootKind == model.RootGlobal {
			hasGlobal = true
			break
		}
	}
	eligible := map[string]model.Root{}
	for _, root := range scan.Roots {
		if root.Kind != model.RootKnownAgent && root.Kind != model.RootUnknownAgent {
			continue
		}
		if ignoredRoot(settings, root) {
			continue
		}
		available := root.Status == model.RootAvailable && root.EntryCount > 0
		if root.ID != current.ID && !available && !(root.SharedSkills && hasGlobal) {
			continue
		}
		eligible[root.ID] = root
	}
	if _, ok := eligible[current.ID]; !ok {
		eligible[current.ID] = current
	}

	ordered := make([]model.Root, 0, 4)
	seen := map[string]bool{}
	seenLogical := map[string]bool{}
	add := func(root model.Root) {
		logical := root.LogicalAgent
		if logical == "" {
			logical = root.Label
		}
		if len(ordered) >= 4 || seen[root.ID] || seenLogical[logical] {
			return
		}
		seen[root.ID] = true
		seenLogical[logical] = true
		ordered = append(ordered, root)
	}
	add(current)
	if len(requested) > 0 {
		for _, id := range requested {
			if root, ok := eligible[id]; ok {
				add(root)
			}
		}
	} else {
		// Default comparison stays small and predictable: the current Agent
		// plus the three primary Agent families. Users can add a fourth Agent
		// explicitly from the picker.
		preferred := []string{"Claude Code", "Codex", "Cursor"}
		for _, label := range preferred {
			best, ok := model.Root{}, false
			for _, root := range eligible {
				if root.LogicalAgent != label && root.Label != label {
					continue
				}
				if !ok || root.EntryCount > best.EntryCount || (root.EntryCount == best.EntryCount && root.ID < best.ID) {
					best, ok = root, true
				}
			}
			if ok {
				add(best)
			}
		}
	}
	return ordered
}

func indexOf(values []string, value string) int {
	for i, item := range values {
		if item == value {
			return i
		}
	}
	return len(values)
}

func groupCandidates(in Input, root model.Root) map[string][]model.Skill {
	groups := map[string][]model.Skill{}
	for _, skill := range in.Scan.Skills {
		local := skill.RootID == root.ID
		linked := visibleLink(in.Scan.AgentLinks, skill.ID, root.ID)
		if !local && !linked && !projectMatch(in.Project, skill) {
			continue
		}
		key := strings.ToLower(skill.Name)
		if !containsSkill(groups[key], skill.ID) {
			groups[key] = append(groups[key], skill)
		}
	}
	for key := range groups {
		sort.SliceStable(groups[key], func(i, j int) bool {
			a, b := groups[key][i], groups[key][j]
			aLocal, bLocal := a.RootID == root.ID, b.RootID == root.ID
			if aLocal != bLocal {
				return aLocal
			}
			return a.Path < b.Path
		})
	}
	return groups
}

func visibleLink(links []model.AgentLink, skillID, rootID string) bool {
	for _, link := range links {
		if link.SkillID == skillID && link.RootID == rootID && link.Visible {
			return true
		}
	}
	return false
}

func projectMatch(project string, skill model.Skill) bool {
	if project == "" || !skill.IsProject {
		return false
	}
	return skill.Path == project || strings.HasPrefix(skill.Path, project+"/")
}

func containsSkill(skills []model.Skill, id string) bool {
	for _, skill := range skills {
		if skill.ID == id {
			return true
		}
	}
	return false
}

func resolveEntry(in Input, candidates []model.Skill) model.XrayEntry {
	entry := model.XrayEntry{Skill: candidates[0], Candidates: candidates}
	if link := visibleLinkEntry(in.Scan.AgentLinks, candidates[0].ID, in.rootID(candidates)); link != nil {
		entry.Link = link
	}
	issue := matchingIssue(in.Scan.Issues, candidates)
	if issue != nil {
		entry.Issue = issue
		entry.Status = model.XrayStatusBroken
		entry.Reason = model.XrayReasonScanFailure
		return entry
	}
	if entry.Link != nil && (entry.Link.Mode == model.LinkBroken || entry.Link.Mode == model.LinkMissing || entry.Link.Mode == model.LinkUnavailable) {
		entry.Status = model.XrayStatusBroken
		entry.Reason = model.XrayReasonBrokenLink
		return entry
	}
	if len(candidates) > 1 {
		entry.Status = model.XrayStatusCandidate
		entry.Reason = model.XrayReasonDuplicateCandidates
		return entry
	}
	if !candidates[0].HasSkillMD {
		entry.Status = model.XrayStatusMissing
		entry.Reason = model.XrayReasonMissingSkillMD
		return entry
	}
	if strings.TrimSpace(candidates[0].Description) == "" {
		entry.Status = model.XrayStatusMissing
		entry.Reason = model.XrayReasonMissingDescription
		return entry
	}
	entry.Status = model.XrayStatusActive
	entry.Reason = model.XrayReasonVisible
	return entry
}

// rootID returns the root that owns the first candidate. Agent links for a
// linked copy are looked up by the caller's Agent root below.
func (in Input) rootID(candidates []model.Skill) string {
	if len(candidates) == 0 {
		return in.AgentID
	}
	root := in.AgentID
	for _, link := range in.Scan.AgentLinks {
		if link.SkillID == candidates[0].ID && link.RootID == root {
			return root
		}
	}
	return root
}

func visibleLinkEntry(links []model.AgentLink, skillID, rootID string) *model.AgentLink {
	for index := range links {
		link := &links[index]
		if link.SkillID == skillID && link.RootID == rootID && link.Visible {
			return link
		}
	}
	return nil
}

func matchingIssue(issues []model.ScanIssue, candidates []model.Skill) *model.ScanIssue {
	for index := range issues {
		issue := &issues[index]
		if issue.Kind != "permission" && issue.Kind != "scan_failure" && issue.Kind != "scan_unavailable" {
			continue
		}
		for _, skill := range candidates {
			if issue.Path == skill.Path {
				return issue
			}
		}
	}
	return nil
}

func relevantInstructions(in Input) []model.InstructionFile {
	result := make([]model.InstructionFile, 0)
	for _, file := range in.Instructions {
		if file.Scope == "global" || matchesAgent(in.RootForAgent(), file.Agent) {
			result = append(result, file)
			continue
		}
		if in.Project != "" && file.Scope == "project" && (file.Project == in.Project || strings.HasPrefix(file.Path, in.Project+"/")) {
			result = append(result, file)
		}
	}
	return result
}

func (in Input) RootForAgent() model.Root {
	for _, root := range in.Scan.Roots {
		if root.ID == in.AgentID {
			return root
		}
	}
	return model.Root{ID: in.AgentID}
}

func matchesAgent(root model.Root, label string) bool {
	return label == root.LogicalAgent || label == root.Label
}

func relevantMCPs(in Input, root model.Root) []model.MCPServer {
	result := make([]model.MCPServer, 0)
	for _, server := range in.MCPServers {
		for _, agent := range server.Agents {
			if matchesAgent(root, agent.Label) {
				result = append(result, server)
				break
			}
		}
	}
	return result
}

func countStatus(entries []model.XrayEntry, status model.XrayStatus) int {
	count := 0
	for _, entry := range entries {
		if entry.Status == status {
			count++
		}
	}
	return count
}

func buildComparison(in Input, roots []model.Root, entries []model.XrayEntry) model.XrayComparison {
	columns := make([]model.XrayAgentColumn, 0, len(roots))
	for _, root := range roots {
		ids := map[string]bool{}
		for _, skill := range in.Scan.Skills {
			if skill.RootID == root.ID || visibleLink(in.Scan.AgentLinks, skill.ID, root.ID) {
				ids[skill.ID] = true
			}
		}
		columns = append(columns, model.XrayAgentColumn{Root: root, SkillCount: len(ids)})
	}
	type compareResource struct {
		name   string
		source string
		skill  *model.Skill
	}
	currentNames := map[string]bool{}
	allNames := map[string]compareResource{}
	for _, entry := range entries {
		key := strings.ToLower(entry.Skill.Name)
		skill := entry.Skill
		currentNames[key] = true
		allNames[key] = compareResource{name: entry.Skill.Name, source: entry.Skill.Source, skill: &skill}
	}
	for _, column := range columns {
		for _, skill := range in.Scan.Skills {
			if skill.RootID != column.Root.ID && !visibleLink(in.Scan.AgentLinks, skill.ID, column.Root.ID) {
				continue
			}
			key := strings.ToLower(skill.Name)
			if existing, ok := allNames[key]; !ok {
				copy := skill
				allNames[key] = compareResource{name: skill.Name, source: skill.Source, skill: &copy}
			} else if existing.skill == nil {
				copy := skill
				existing.skill = &copy
				allNames[key] = existing
			}
		}
	}
	keys := make([]string, 0, len(allNames))
	for key := range allNames {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if currentNames[keys[i]] != currentNames[keys[j]] {
			return currentNames[keys[i]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > 16 {
		keys = keys[:16]
	}
	rows := make([]model.XrayCompareRow, 0, len(keys))
	for _, key := range keys {
		resource := allNames[key]
		row := model.XrayCompareRow{Name: resource.name, Source: resource.source, Skill: resource.skill}
		for _, column := range columns {
			candidates := candidatesForRoot(in, column.Root.ID, key)
			status := model.XrayStatusAbsent
			if len(candidates) > 0 {
				status = resolveEntry(in.withRoot(column.Root.ID), candidates).Status
			}
			row.Cells = append(row.Cells, model.XrayCompareCell{Root: column.Root, Status: status})
		}
		rows = append(rows, row)
	}
	return model.XrayComparison{Columns: columns, Rows: rows}
}

func candidatesForRoot(in Input, rootID, name string) []model.Skill {
	result := make([]model.Skill, 0)
	for _, skill := range in.Scan.Skills {
		if strings.ToLower(skill.Name) != name {
			continue
		}
		if skill.RootID == rootID || visibleLink(in.Scan.AgentLinks, skill.ID, rootID) {
			result = append(result, skill)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i], result[j]
		aLocal, bLocal := a.RootID == rootID, b.RootID == rootID
		if aLocal != bLocal {
			return aLocal
		}
		return a.Path < b.Path
	})
	return result
}

func (in Input) withRoot(rootID string) Input {
	in.AgentID = rootID
	return in
}
