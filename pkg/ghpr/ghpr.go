package ghpr

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
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
	RepoName     string
	Repo         *git.Repository
	Path         string
	Auth         http.BasicAuth
	Pr           int
	GitHubClient *github.Client
	MergeSHA     string
}

// MakeGithubPR creates a new GithubPR struct with all the necessary state to clone, commit, raise a PR
// and merge
func MakeGithubPR(repoName string, creds Credentials) GithubPR {
	return GithubPR{RepoName: repoName, Auth: http.BasicAuth{Username: creds.Username, Password: creds.Token}}
}

// Clone shallow clones a GitHub repository to a temporary directory
func (r *GithubPR) Clone() error {
	tempDir, err := ioutil.TempDir(".", "repo_")
	if err != nil {
		return err
	}
	r.Path = tempDir

	url := fmt.Sprintf("https://github.com/" + r.RepoName)

	gitRepo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
		Auth:  &r.Auth,
	})
	if err != nil {
		os.RemoveAll(tempDir)
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
