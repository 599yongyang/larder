package model

import "time"

// SkillEditPlan is a preview of editing an existing SKILL.md.
type SkillEditPlan struct {
	ID             string    `json:"id"`
	SkillID        string    `json:"skillId"`
	Path           string    `json:"path"`
	Content        string    `json:"content"`
	CurrentContent string    `json:"currentContent,omitempty"`
	Exists         bool      `json:"exists"`
	Warnings       []string  `json:"warnings,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// SkillCreatePlan is a preview of creating a new shared Skill.
type SkillCreatePlan struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Content   string    `json:"content"`
	Warnings  []string  `json:"warnings,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// SkillRenamePlan is a preview of renaming a Skill directory.
type SkillRenamePlan struct {
	ID        string    `json:"id"`
	SkillID   string    `json:"skillId"`
	Name      string    `json:"name"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	CreatedAt time.Time `json:"createdAt"`
}
