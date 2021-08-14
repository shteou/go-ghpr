package ghpr

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// UpateFunc
type UpdateFunc func(w *git.Worktree) (string, *object.Signature, error)

type Credentials struct {
	Username string
	Token    string
}

type Author struct {
	Name  string
	Email string
}

// GithubPR GitHubPR is a container for all
type GithubPR struct {
	Repo *git.Repository
	Path string
	Auth http.BasicAuth
}

// Clone clones a GitHub repository to a temporary directory
func Clone(repo string, creds Credentials) (*GithubPR, error) {
	tempDir, err := ioutil.TempDir(".", "repo_")
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://github.com/" + repo)

	r, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
		Auth:  &http.BasicAuth{Username: creds.Username, Password: creds.Token},
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	return &GithubPR{Repo: r, Path: tempDir, Auth: http.BasicAuth{Username: creds.Username, Password: creds.Token}}, nil
}

// CreateCommit CreateCommit Creates a commit via the passedd UpdateFunc
func PushCommit(r *GithubPR, branchName string, fn UpdateFunc) error {
	headRef, err := r.Repo.Head()
	if err != nil {
		return err
	}

	branchRef := fmt.Sprintf("refs/heads/%s", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName(branchRef), headRef.Hash())
	err = r.Repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	w, err := r.Repo.Worktree()
	if err != nil {
		return err
	}

	w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName(branchName)})

	commitMessage, author, err := fn(w)
	if err != nil {
		return err
	}

	// If no commit time is set (i.e. defaulted to the epoch), use the current time
	if author.When.Equal(time.Time{}) {
		author.When = time.Now()
	}

	_, err = w.Commit(commitMessage, &git.CommitOptions{Author: author})
	if err != nil {
		return err
	}

	branchRef = fmt.Sprintf("refs/remotes/origin/%s", branchName)
	ref = plumbing.NewHashReference(plumbing.ReferenceName(branchRef), headRef.Hash())
	err = r.Repo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	err = r.Repo.Push(&git.PushOptions{
		Auth: &r.Auth,
	})
	return err
}

func Close(r *GithubPR) error {
	return os.RemoveAll(r.Path)
}
