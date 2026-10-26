package model

import "time"

// Command is a read-only Agent custom slash command or prompt template.
type Command struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	ArgumentHint string    `json:"argumentHint,omitempty"`
	AllowedTools []string  `json:"allowedTools,omitempty"`
	Model        string    `json:"model,omitempty"`
	Agent        string    `json:"agent"`
	RootPath     string    `json:"rootPath"`
	Path         string    `json:"path"`
	Format       string    `json:"format"`
	Scope        string    `json:"scope"`
	Project      string    `json:"project,omitempty"`
	Size         int64     `json:"size"`
	ModifiedAt   time.Time `json:"modifiedAt"`
	Hash         string    `json:"hash"`
}

// CommandTarget describes a writable Agent command location.
type CommandTarget struct {
	Agent    string `json:"agent"`
	Scope    string `json:"scope"`
	Project  string `json:"project,omitempty"`
	RootPath string `json:"rootPath"`
	Format   string `json:"format"`
}

// CommandWriteRequest describes a create, update or conversion operation.
type CommandWriteRequest struct {
	Path       string `json:"path,omitempty"`
	Agent      string `json:"agent,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Project    string `json:"project,omitempty"`
	RootPath   string `json:"rootPath,omitempty"`
	Name       string `json:"name,omitempty"`
	Content    string `json:"content,omitempty"`
	SourcePath string `json:"sourcePath,omitempty"`
}

type CommandWritePlan struct {
	ID             string        `json:"id"`
	Operation      string        `json:"operation"`
	Target         CommandTarget `json:"target"`
	Name           string        `json:"name"`
	Path           string        `json:"path"`
	Content        string        `json:"content"`
	CurrentContent string        `json:"currentContent,omitempty"`
	Exists         bool          `json:"exists"`
	Warnings       []string      `json:"warnings,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
}

// CommandDeletePlan is a preview of deleting an existing command file.
type CommandDeletePlan struct {
	ID             string        `json:"id"`
	Target         CommandTarget `json:"target"`
	Name           string        `json:"name"`
	Path           string        `json:"path"`
	CurrentContent string        `json:"currentContent,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
}
