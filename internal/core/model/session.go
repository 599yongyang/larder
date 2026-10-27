package model

import "time"

// Session is a read-only summary of an Agent session associated with a
// project directory. Larder keeps only metadata here; transcript content is
// deliberately not part of the project overview.
type Session struct {
	ID        string    `json:"id"`
	Agent     string    `json:"agent"`
	CWD       string    `json:"cwd"`
	Title     string    `json:"title,omitempty"`
	Model     string    `json:"model,omitempty"`
	Path      string    `json:"path"`
	StartedAt time.Time `json:"startedAt"`
	LastAt    time.Time `json:"lastAt"`
	Messages  int       `json:"messages"`
	TokensIn  int       `json:"tokensIn"`
	TokensOut int       `json:"tokensOut"`
}

// SessionMessage is an on-demand preview of one transcript message. It is not
// part of the session index and is only returned when the user asks for it.
type SessionMessage struct {
	Role string    `json:"role"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// ResourceUsage is evidence from local sessions that a Skill or MCP tool was
// actually invoked. Names are normalized from the Agent's tool calls.
type ResourceUsage struct {
	Kind   string    `json:"kind"`
	Name   string    `json:"name"`
	Count  int       `json:"count"`
	LastAt time.Time `json:"lastAt"`
}
