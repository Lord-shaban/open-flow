// Package provider defines vendor-neutral contracts. No concrete adapter ships in M0.
package provider

import (
	"context"
	"time"
)

type Capability string

const (
	ImageGeneration Capability = "image_generation"
	VideoGeneration Capability = "video_generation"
)

type Model struct {
	ID           string       `json:"id"`
	ProviderID   string       `json:"provider_id"`
	DisplayName  string       `json:"display_name"`
	Capabilities []Capability `json:"capabilities"`
	Region       string       `json:"region,omitempty"`
	Selectable   bool         `json:"selectable"`
	Reason       string       `json:"reason"`
}

type Submission struct {
	GenerationID  string
	ModelID       string
	CredentialRef string
	PromptRef     string // Resolve sensitive prompt inside activity, outside Temporal history.
	Capability    Capability
	Prompt        string `json:"-"` // Activity-local only; never a workflow argument/result.
	AspectRatio   string
}

type Artifact struct {
	ProviderReference string // Internal only; download through allowlisted provider transport.
	MIMEType          string
	Data              []byte `json:"-"` // Activity-local only; private storage precedes workflow completion.
}

type Operation struct {
	ID        string
	Done      bool
	Artifacts []Artifact
}

type Health struct {
	Available bool
	CheckedAt time.Time
	Latency   time.Duration
}

type Adapter interface {
	ID() string
	DiscoverModels(context.Context, string) ([]Model, error)
	TestConnection(context.Context, string) (Health, error)
	Submit(context.Context, Submission) (Operation, error)
	Poll(context.Context, string, string) (Operation, error)
	Cancel(context.Context, string, string) error
}

type ErrorCategory string

const (
	Authentication      ErrorCategory = "authentication"
	Billing             ErrorCategory = "billing"
	Quota               ErrorCategory = "quota"
	RateLimited         ErrorCategory = "rate_limited"
	Transient           ErrorCategory = "transient"
	PolicyBlocked       ErrorCategory = "policy_blocked"
	AmbiguousSubmission ErrorCategory = "ambiguous_submission"
	Unsupported         ErrorCategory = "unsupported"
)

// Error contains a sanitized message; vendor response bodies and credentials stay private.
type Error struct {
	Category        ErrorCategory
	Message         string
	RetryAfter      time.Duration
	AcceptanceKnown bool
}

func (e *Error) Error() string { return string(e.Category) + ": " + e.Message }
