package ghpr

import (
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// UpdateFunc is a callback function which should create a series of changes
// to the git WorkTree. These changes will be automatically committed on successful
// return by the PushCommit function
type UpdateFunc func(w *git.Worktree) (string, *object.Signature, error)

// Credentials represents a GitHub username and PAT
type Credentials struct {
	Username string
	Token    string
}

// Author represents information about the creator of a commit
type Author struct {
	Name  string
	Email string
}

// BackoffStrategy provides describes how to wait for a GitHub status check
type BackoffStrategy struct {
	// The initial wait time
	MinPollTime time.Duration
	// The max wait time when polling for a status
	MaxPollTime time.Duration
	// The poll time will be multiplied by this (up to max)
	PollBackoffFactor float32
}

// Check represents a GitHub action result or status
type Check struct {
	// Name of the check, e.g. "Semantic Pull Request"
	Name string
	// CheckType the type of check, either "action" or "status"
	CheckType string
}
