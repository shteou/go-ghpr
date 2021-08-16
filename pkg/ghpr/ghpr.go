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
	RepoName     string
	Repo         *git.Repository
	Path         string
	Auth         http.BasicAuth
	Pr           int
	GitHubClient *github.Client
}

func MakeGithubPR(repoName string, creds Credentials) GithubPR {
	return GithubPR{RepoName: repoName, Auth: http.BasicAuth{Username: creds.Username, Password: creds.Token}}
}

// Clone clones a GitHub repository to a temporary directory
func (r *GithubPR) Clone() error {
	tempDir, err := ioutil.TempDir(".", "repo_")
	if err != nil {
		return err
	}

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
	r.Path = tempDir

	return nil
}

// CreateCommit CreateCommit Creates a commit via the passedd UpdateFunc
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

func (r *GithubPR) WaitForPR(statusContext string) error {
	r.makeGitHubClient()

	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	pr, _, err := r.GitHubClient.PullRequests.Get(context.Background(), owner, repo, r.Pr)
	if err != nil {
		return err
	}

	c1 := make(chan error, 1)
	go func() {
		println("Waiting for PR to become mergeable")
		for {
			time.Sleep(time.Second * 2)
			statuses, _, err := r.GitHubClient.Repositories.ListStatuses(context.Background(), owner, repo,
				*pr.Head.Ref, &github.ListOptions{PerPage: 20})

			if err != nil {
				fmt.Println("Found an error listing check runs for ref")
				c1 <- err
			}

			if statuses != nil {
				for i := 0; i < len(statuses); i++ {
					if statuses[i].GetContext() == statusContext && statuses[i].GetState() == "success" {
						break
					}
				}
			}
			c1 <- nil
		}
	}()

	select {
	case err := <-c1:
		if err != nil {
			return err
		}
	case <-time.After(60 * time.Second):
		return errors.New("timed out waiting for PR to become mergeable")
	}

	return nil
}

func (r *GithubPR) MergePR() error {
	r.makeGitHubClient()

	owner := strings.Split(r.RepoName, "/")[0]
	repo := strings.Split(r.RepoName, "/")[1]

	pr, _, err := r.GitHubClient.PullRequests.Get(context.Background(), owner, repo, r.Pr)
	if err != nil {
		return err
	}

	if pr.Mergeable != nil && *pr.Mergeable {
		println("PR is mergeable, proceeding to merge")
		_, _, err := r.GitHubClient.PullRequests.Merge(context.Background(), owner, repo, *pr.Number, "", &github.PullRequestOptions{MergeMethod: "squash", CommitTitle: "Ooh, neat"})
		if err != nil {
			return err
		}
		println("Successfully merged PR")
	} else {
		return errors.New("PR is not mergeable")
	}
	return nil
}

func (r *GithubPR) Close() error {
	return os.RemoveAll(r.Path)
}
