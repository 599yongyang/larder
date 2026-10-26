package model

import "time"

// XrayStatus is the static resolution state of a resource for one Agent.
// It deliberately does not claim to know the Agent's runtime load order.
type XrayStatus string

const (
	XrayStatusActive    XrayStatus = "active"
	XrayStatusCandidate XrayStatus = "candidate"
	XrayStatusMissing   XrayStatus = "missing"
	XrayStatusBroken    XrayStatus = "broken"
	XrayStatusAbsent    XrayStatus = "absent"
)

type XrayReason string

const (
	XrayReasonVisible             XrayReason = "visible"
	XrayReasonDuplicateCandidates XrayReason = "duplicate_candidates"
	XrayReasonMissingSkillMD      XrayReason = "missing_skill_md"
	XrayReasonMissingDescription  XrayReason = "missing_description"
	XrayReasonBrokenLink          XrayReason = "broken_link"
	XrayReasonNotLoaded           XrayReason = "not_loaded"
	XrayReasonScanFailure         XrayReason = "scan_failure"
)

// XrayEntry is one resource after static resolution for an Agent.
type XrayEntry struct {
	Skill      Skill      `json:"skill"`
	Status     XrayStatus `json:"status"`
	Reason     XrayReason `json:"reason"`
	Candidates []Skill    `json:"candidates"`
	Link       *AgentLink `json:"link,omitempty"`
	Issue      *ScanIssue `json:"issue,omitempty"`
}

type XrayBudget struct {
	SkillTokens       int  `json:"skillTokens"`
	InstructionTokens int  `json:"instructionTokens"`
	KnownTokens       int  `json:"knownTokens"`
	Unmodeled         bool `json:"unmodeled"`
}

type XrayCompareCell struct {
	Root   Root       `json:"root"`
	Status XrayStatus `json:"status"`
}

type XrayCompareRow struct {
	Name   string            `json:"name"`
	Source string            `json:"source,omitempty"`
	Skill  *Skill            `json:"skill,omitempty"`
	Cells  []XrayCompareCell `json:"cells"`
}

type XrayAgentColumn struct {
	Root       Root `json:"root"`
	SkillCount int  `json:"skillCount"`
}

type XrayComparison struct {
	Columns []XrayAgentColumn `json:"columns"`
	Rows    []XrayCompareRow  `json:"rows"`
}

type XrayReport struct {
	SchemaVersion  string            `json:"schemaVersion"`
	AgentID        string            `json:"agentId"`
	AgentLabel     string            `json:"agentLabel"`
	Project        string            `json:"project,omitempty"`
	Root           Root              `json:"root"`
	Entries        []XrayEntry       `json:"entries"`
	ActiveCount    int               `json:"activeCount"`
	MissingCount   int               `json:"missingCount"`
	CandidateCount int               `json:"candidateCount"`
	BrokenCount    int               `json:"brokenCount"`
	Budget         XrayBudget        `json:"budget"`
	Instructions   []InstructionFile `json:"instructions"`
	MCPServers     []MCPServer       `json:"mcps"`
	Contributors   []XrayEntry       `json:"contributors"`
	Compare        XrayComparison    `json:"compare"`
	GeneratedAt    time.Time         `json:"generatedAt"`
}
