package ghpr

import (
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage"
)

// goGit provides an interface for to go-git methods in use by this module
// This is interface is not exported.
type goGit interface {
	Clone(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error)
}

// realGoGit is a go-git backed implementation of the GoGit interface
type realGoGit struct {
}

func (g realGoGit) Clone(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error) {
	return git.Clone(s, worktree, o)
}
