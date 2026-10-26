package model

import "time"

// SkillAuditEntry is one upstream security review for a Skill.
type SkillAuditEntry struct {
	Provider   string    `json:"provider"`
	Slug       string    `json:"slug,omitempty"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary,omitempty"`
	RiskLevel  string    `json:"riskLevel,omitempty"`
	AuditedAt  time.Time `json:"auditedAt,omitempty"`
	Categories []string  `json:"categories,omitempty"`
}

// SkillAudit summarizes upstream audits for one installed Skill.
type SkillAudit struct {
	SkillID         string            `json:"skillId"`
	Source          string            `json:"source,omitempty"`
	Skill           string            `json:"skill,omitempty"`
	Available       bool              `json:"available"`
	Verdict         string            `json:"verdict"` // pass, warn, fail, unreviewed
	RiskLevel       string            `json:"riskLevel,omitempty"`
	Entries         []SkillAuditEntry `json:"entries,omitempty"`
	CheckedAt       time.Time         `json:"checkedAt,omitempty"`
	Reason          string            `json:"reason,omitempty"`
	LocalHash       string            `json:"localHash,omitempty"`
	LocalChanged    bool              `json:"localChanged,omitempty"`
	LocalBaselineAt time.Time         `json:"localBaselineAt,omitempty"`
}
