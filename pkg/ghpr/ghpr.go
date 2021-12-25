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

// StatusWaitStrategy describes how to wait for a GitHub status check
type StatusWaitStrategy struct {
	// The initial wait time
	MinPollTime time.Duration
	// The max wait time when polling for a status
	MaxPollTime time.Duration
	// The poll time will be multiplied by this (up to max)
	PollBackoffFactor float32
	// The total timeout for the wait operation
	Timeout time.Duration
	// WaitStatusContext indicates the name of the status check to wait for
	WaitStatusContext string
}
