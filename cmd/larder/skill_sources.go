package larder

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"larder/internal/core/skills"
)

type skillSourceFile struct {
	Skills map[string]skills.SourceOverride `json:"skills"`
}

func (s *AppService) loadSkillSources() {
	s.skillSources = map[string]skills.SourceOverride{}
	raw, err := os.ReadFile(s.skillSourcesPath)
	if err != nil {
		s.core.SetSkillSourceOverrides(s.skillSources)
		return
	}
	var file skillSourceFile
	if json.Unmarshal(raw, &file) == nil && file.Skills != nil {
		s.skillSources = file.Skills
	}
	s.core.SetSkillSourceOverrides(s.skillSources)
}

func (s *AppService) saveSkillSources() error {
	raw, err := json.MarshalIndent(skillSourceFile{Skills: s.skillSources}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.skillSourcesPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.skillSourcesPath, raw, 0o600)
}

// BindSkillSource records a user-supplied source for an otherwise unresolved
// skill. Overrides take precedence over local lockfiles and source indexes.
func (s *AppService) BindSkillSource(path, source, sourceURL, skillPath string) error {
	path = strings.TrimSpace(path)
	source = strings.TrimSpace(source)
	sourceURL = strings.TrimSpace(sourceURL)
	skillPath = strings.Trim(strings.TrimSpace(skillPath), "/")
	if path == "" {
		return errors.New("skill path is required")
	}
	if source == "" && sourceURL == "" {
		return errors.New("source or source URL is required")
	}
	if sourceURL == "" && strings.Contains(source, "/") && !strings.HasPrefix(source, "/") {
		sourceURL = "https://github.com/" + source
	}
	if s.skillSources == nil {
		s.skillSources = map[string]skills.SourceOverride{}
	}
	s.skillSources[filepath.Clean(path)] = skills.SourceOverride{Source: source, SourceType: "manual", SourceURL: sourceURL, SkillPath: skillPath}
	if err := s.saveSkillSources(); err != nil {
		return err
	}
	s.core.SetSkillSourceOverrides(s.skillSources)
	return nil
}

func (s *AppService) ClearSkillSource(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("skill path is required")
	}
	delete(s.skillSources, filepath.Clean(path))
	if err := s.saveSkillSources(); err != nil {
		return err
	}
	s.core.SetSkillSourceOverrides(s.skillSources)
	return nil
}
