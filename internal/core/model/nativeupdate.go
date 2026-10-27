package model

import "time"

// NativeCLIUpdateInfo describes an update check for a CLI that has its own
// native updater instead of Homebrew/npm/winget metadata.
type NativeCLIUpdateInfo struct {
	CLIID         string    `json:"cliId"`
	Current       string    `json:"current,omitempty"`
	Latest        string    `json:"latest,omitempty"`
	Available     bool      `json:"available"`
	CanUpdate     bool      `json:"canUpdate"`
	UpdateCommand string    `json:"updateCommand,omitempty"`
	Error         string    `json:"error,omitempty"`
	CheckedAt     time.Time `json:"checkedAt"`
}
