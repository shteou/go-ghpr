package ghpr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
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

// GithubPR GitHubPR is a container for all necessary state
type GithubPR struct {
	Auth         http.BasicAuth
	Filesystem   billy.Filesystem
	GitHubClient *github.Client
	MergeSHA     string
	Path         string
	Pr           int
	Repo         *git.Repository
	RepoName     string
}

// MakeGithubPR creates a new GithubPR struct with all the necessary state to clone, commit, raise a PR
// and merge. The repository will be cloned to a temporary directory in the given filesystem. If no
// filesystem is provided an OS backed fs will be used
func MakeGithubPR(repoName string, creds Credentials, fs *billy.Filesystem) (*GithubPR, error) {
	if fs == nil {
		defaultFs := osfs.New(".")
		fs = &defaultFs
	}

	tempDir, err := util.TempDir(*fs, ".", "repo_")
	if err != nil {
		return nil, err
	}

	*fs, err = (*fs).Chroot(tempDir)
	print((*fs).Stat("."))
	if err != nil {
		return nil, err
	}

	return &GithubPR{
		RepoName:   repoName,
		Filesystem: *fs,
		Auth:       http.BasicAuth{Username: creds.Username, Password: creds.Token},
		Path:       tempDir,
	}, nil
}

// Clone shallow clones a GitHub repository to a temporary directory
func (r *GithubPR) Clone() error {
	url := fmt.Sprintf("https://github.com/" + r.RepoName)

	storageWorkTree, err := r.Filesystem.Chroot(".git")
	if err != nil {
		return err
	}

	// Pass a defafult LRU object cache, as per git.PlainClone's implementation
	gitRepo, err := git.Clone(
		filesystem.NewStorage(storageWorkTree, cache.NewObjectLRUDefault()),
		r.Filesystem,
		&git.CloneOptions{
			Depth: 1,
			URL:   url,
			Auth:  &r.Auth})

	if err != nil {
		return err
	}
	r.Repo = gitRepo

	return nil
}

// PushCommit creates a commit for the Worktree changes made by the UpdateFunc parameter
// and pushes that branch to the remote origin server
func (r *GithubPR) PushCommit(branchName string, fn UpdateFunc) error {
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

func (r *GithubPR) makeGitHubClient() {
	if r.GitHubClient == nil {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: r.Auth.Password},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		r.GitHubClient = github.NewClient(tc)
	}
}

// RaisePR creates a pull request from the sourceBranch (HEAD) to the targetBranch (base)
func (r *GithubPR) RaisePR(sourceBranch string, targetBranch string, title string, body string) error {
	r.makeGitHubClient()
	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	pr, _, err := r.GitHubClient.PullRequests.Create(context.Background(),
		owner, repo,
		&github.NewPullRequest{
			Title: &title,
			Head:  &sourceBranch,
			Base:  &targetBranch,
			Body:  &body})
	if err != nil {
		return err
	}

	r.Pr = *pr.Number

	return err
}

func (r *GithubPR) waitForStatus(shaRef string, owner string, repo string, statusContext string) error {
	c1 := make(chan error, 1)
	go func() {
		fmt.Printf("Waiting for %s to become mergeable\n", shaRef)
		for {
			time.Sleep(time.Second * 2)
			statuses, _, err := r.GitHubClient.Repositories.ListStatuses(context.Background(), owner, repo,
				shaRef, &github.ListOptions{PerPage: 20})

			if err != nil {
				c1 <- err
				return
			}

			if statuses != nil {
				for i := 0; i < len(statuses); i++ {
					context := statuses[i].GetContext()
					state := statuses[i].GetState()

					if context == statusContext {
						if state == "success" {
							c1 <- nil
							return
						}
						if state == "failure" || state == "error" {
							c1 <- errors.New("target status check is in a failed state, aborting")
							return
						}
					}
				}
			}
		}
	}()

	select {
	case err := <-c1:
		return err
	case <-time.After(60 * time.Minute):
		return errors.New("timed out waiting for PR to become mergeable")
	}
}

// WaitForPR waits until the raised PR passes the supplied status check. It returns
// an error if a failed or errored state is encountered
func (r *GithubPR) WaitForPR(statusContext string) error {
	r.makeGitHubClient()

	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	pr, _, err := r.GitHubClient.PullRequests.Get(context.Background(), owner, repo, r.Pr)
	if err != nil {
		return err
	}

	fmt.Printf("HEAD sha is %s\n", *pr.Head.SHA)
	return r.waitForStatus(*pr.Head.SHA, owner, repo, statusContext)

}

// MergePR merges a PR, provided it is in a mergeable state, otherwise returning
// an error
func (r *GithubPR) MergePR() error {
	r.makeGitHubClient()

	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	pr, _, err := r.GitHubClient.PullRequests.Get(context.Background(), owner, repo, r.Pr)
	if err != nil {
		return err
	}

	if pr.Mergeable != nil && *pr.Mergeable {
		merge, _, err := r.GitHubClient.PullRequests.Merge(context.Background(), owner, repo, *pr.Number, "", &github.PullRequestOptions{MergeMethod: "merge"})
		if err != nil {
			return err
		}
		r.MergeSHA = *merge.SHA
	} else {
		return errors.New("PR is not mergeable")
	}
	return nil
}

// WaitForMergeCommit waits for the merge commit to receive a successful state
// for the supplied status check. It returns an error if a failed or errored
// state is encountered
func (r *GithubPR) WaitForMergeCommit(statusContext string) error {
	r.makeGitHubClient()

	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	return r.waitForStatus(r.MergeSHA, owner, repo, statusContext)
}

// Close cleans any local files for the change
func (r *GithubPR) Close() error {
	return os.RemoveAll(r.Path)
}

func (r *GithubPR) Create(branchName string, targetBranch string, prStatusContext string, masterStatusContext string, fn UpdateFunc) error {
	err := r.Clone()
	defer r.Close()
	if err != nil {
		return err
	}

	err = r.PushCommit(branchName, fn)
	if err != nil {
		return err
	}

	stuff := "test"
	err = r.RaisePR(branchName, targetBranch, stuff, "")
	if err != nil {
		return err
	}

	err = r.WaitForPR(prStatusContext)
	if err != nil {
		return err
	}

	err = r.MergePR()
	if err != nil {
		return err
	}

	return r.WaitForMergeCommit(masterStatusContext)
}
