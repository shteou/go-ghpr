package ghpr

import (
	"fmt"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/pkg/errors"
)

type Change struct {
	Branch     string
	repo       Repo
	updateFunc UpdateFunc
	creds      Credentials
}

func NewChange(repo Repo, branch string, creds Credentials, fn UpdateFunc) Change {
	return Change{
		Branch:     branch,
		repo:       repo,
		updateFunc: fn,
		creds:      creds,
	}
}

func (c *Change) Push() error {
	headRef, err := c.repo.repo.Head()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve HEAD ref of repository")
	}

	branchRef := fmt.Sprintf("refs/heads/%s", c.Branch)
	ref := plumbing.NewHashReference(plumbing.ReferenceName(branchRef), headRef.Hash())
	err = c.repo.repo.Storer.SetReference(ref)
	if err != nil {
		return errors.Wrap(err, "failed to set reference for new branch")
	}

	w, err := c.repo.repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "failed to fetch Worktree for cloned repository")
	}

	w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(c.Branch)})

	commitMessage, author, err := c.updateFunc(w)
	if err != nil {
		return errors.Wrap(err, "failed to update Worktree with changes")
	}

	// If no commit time is set (i.e. defaulted to the epoch), use the current time
	if author.When.Equal(time.Time{}) {
		author.When = time.Now()
	}

	_, err = w.Commit(commitMessage, &git.CommitOptions{Author: author})
	if err != nil {
		return errors.Wrap(err, "failed to commit changes")
	}

	branchRef = fmt.Sprintf("refs/remotes/origin/%s", c.Branch)
	ref = plumbing.NewHashReference(plumbing.ReferenceName(branchRef), headRef.Hash())
	err = c.repo.repo.Storer.SetReference(ref)
	if err != nil {
		return errors.Wrap(err, "failed to set reference for remote branch")
	}

	auth := http.BasicAuth{Username: c.creds.Username, Password: c.creds.Token}
	err = c.repo.repo.Push(&git.PushOptions{
		Auth: &auth,
	})
	if err != nil {
		return errors.Wrap(err, "failed to push branch to remote repository")
	}
	return nil
}
