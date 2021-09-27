package ghpr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
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
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/google/go-github/github"
	"github.com/jpillora/backoff"
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
	auth         http.BasicAuth
	filesystem   billy.Filesystem
	git          goGit
	gitHubClient *github.Client
	mergeSHA     string
	path         string
	pr           int
	gitRepo      *git.Repository
	owner        string
	repo         string
}

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

// MakeGithubPR creates a new GithubPR struct with all the necessary state to clone, commit, raise a PR
// and merge. The repository will be cloned to a temporary directory in the current directory
func MakeGithubPR(repoName string, creds Credentials) (*GithubPR, error) {
	fs := osfs.New(".")
	return makeGithubPR(repoName, creds, &fs, realGoGit{})
}

// makeGithubPR is an internal function for creating a GithubPR instance. It allows injecting a mock filesystem
// and go-git implementation
func makeGithubPR(repoName string, creds Credentials, fs *billy.Filesystem, gogit goGit) (*GithubPR, error) {
	// A loose regex for a format of <user|org>/<repository>
	// Match one or more non-slash characters, followed by a slash,
	// followed by one or morer non-slash characters
	matched, err := regexp.MatchString("^[^/]+/[^/]+$", repoName)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, errors.New("invalid repository name supplied")
	}

	owner := strings.Split(repoName, "/")[0]
	repo := strings.Split(repoName, "/")[1]

	tempDir, err := util.TempDir(*fs, ".", "repo_")
	if err != nil {
		return nil, err
	}

	*fs, err = (*fs).Chroot(tempDir)
	if err != nil {
		return nil, err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: creds.Token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	return &GithubPR{
		filesystem:   *fs,
		auth:         http.BasicAuth{Username: creds.Username, Password: creds.Token},
		path:         tempDir,
		gitHubClient: github.NewClient(tc),
		git:          gogit,
		repo:         repo,
		owner:        owner,
	}, nil
}

// Clone shallow clones the GitHub repository
func (g *GithubPR) Clone() error {
	url := fmt.Sprintf("https://github.com/" + g.owner + "/" + g.repo)

	storageWorkTree, err := g.filesystem.Chroot(".git")
	if err != nil {
		return err
	}

	// Pass a defafult LRU object cache, as per git.PlainClone's implementation
	g.gitRepo, err = g.git.Clone(
		filesystem.NewStorage(storageWorkTree, cache.NewObjectLRUDefault()),
		g.filesystem,
		&git.CloneOptions{
			Depth: 1,
			URL:   url,
			Auth:  &g.auth})

	if err != nil {
		return err
	}

	return nil
}

// PushCommit creates a commit for the Worktree changes made by the UpdateFunc parameter
// and pushes that branch to the remote origin server
func (g *GithubPR) PushCommit(branchName string, fn UpdateFunc) error {
	headRef, err := g.gitRepo.Head()
	if err != nil {
		return err
	}

	branchRef := fmt.Sprintf("refs/heads/%s", branchName)
	ref := plumbing.NewHashReference(plumbing.ReferenceName(branchRef), headRef.Hash())
	err = g.gitRepo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	w, err := g.gitRepo.Worktree()
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
	err = g.gitRepo.Storer.SetReference(ref)
	if err != nil {
		return err
	}

	err = g.gitRepo.Push(&git.PushOptions{
		Auth: &g.auth,
	})
	return err
}

// RaisePR creates a pull request from the sourceBranch (HEAD) to the targetBranch (base)
func (g *GithubPR) RaisePR(sourceBranch string, targetBranch string, title string, body string) error {
	pr, _, err := g.gitHubClient.PullRequests.Create(context.Background(),
		g.owner, g.repo,
		&github.NewPullRequest{
			Title: &title,
			Head:  &sourceBranch,
			Base:  &targetBranch,
			Body:  &body})
	if err != nil {
		return err
	}

	g.pr = *pr.Number

	return err
}

func (g *GithubPR) waitForStatus(shaRef string, owner string, repo string, statusContext string, c chan error) {
	b := &backoff.Backoff{
		Min:    20 * time.Second,
		Max:    60 * time.Second,
		Factor: 1.02,
		Jitter: true,
	}

	for {
		fmt.Printf("Waiting for %s to become mergeable\n", shaRef)
		statuses, _, err := g.gitHubClient.Repositories.ListStatuses(context.Background(), owner, repo,
			shaRef, &github.ListOptions{PerPage: 20})

		if err != nil {
			c <- err
			return
		}

		for i := 0; i < len(statuses); i++ {
			s := statuses[i]
			if s.GetContext() != statusContext {
				continue
			}

			if s.GetState() == "success" {
				c <- nil
				return
			}
			if s.GetState() == "failure" || s.GetState() == "error" {
				c <- errors.New("target status check is in a failed state, aborting")
				return
			}
		}
		time.Sleep(b.Duration())
	}
}

func (g *GithubPR) waitForStatusWithTimeout(shaRef string, owner string, repo string, statusContext string, timeout time.Duration) error {
	c := make(chan error, 10)
	go func() {
		g.waitForStatus(shaRef, owner, repo, statusContext, c)
	}()

	select {
	case err := <-c:
		return err
	case <-time.After(timeout):
		return errors.New("timed out waiting for PR to become mergeable")
	}
}

// WaitForPR waits until the raised PR passes the supplied status check. It returns
// an error if a failed or errored state is encountered
func (g *GithubPR) WaitForPR(statusContext string, timeout time.Duration) error {
	pr, _, err := g.gitHubClient.PullRequests.Get(context.Background(), g.owner, g.repo, g.pr)
	if err != nil {
		return err
	}

	fmt.Printf("HEAD sha is %s\n", *pr.Head.SHA)
	return g.waitForStatusWithTimeout(*pr.Head.SHA, g.owner, g.repo, statusContext, timeout)

}

// MergePR merges a PR, provided it is in a mergeable state, otherwise returning
// an error
func (g *GithubPR) MergePR() error {
	pr, _, err := g.gitHubClient.PullRequests.Get(context.Background(), g.owner, g.repo, g.pr)
	if err != nil {
		return err
	}

	if pr.Mergeable != nil && *pr.Mergeable {
		merge, _, err := g.gitHubClient.PullRequests.Merge(context.Background(), g.owner, g.repo, *pr.Number, "", &github.PullRequestOptions{MergeMethod: "merge"})
		if err != nil {
			return err
		}
		g.mergeSHA = *merge.SHA
	} else {
		return errors.New("PR is not mergeable")
	}
	return nil
}

// WaitForMergeCommit waits for the merge commit to receive a successful state
// for the supplied status check. It returns an error if a failed or errored
// state is encountered
func (g *GithubPR) WaitForMergeCommit(statusContext string, timeout time.Duration) error {
	return g.waitForStatusWithTimeout(g.mergeSHA, g.owner, g.repo, statusContext, timeout)
}

// Close removes the cloned repository from the filesystem
func (g *GithubPR) Close() error {
	return os.RemoveAll(g.path)
}

// Create performs a full flow. It clones a repository, allows you to make a commit to a branch on
// that repository, pushes the branch and creates a Pull Request. It then waits for the PR to become
// mergeable, merges it and waits for the merge commit to become mergeable.
func (g *GithubPR) Create(branchName string, targetBranch string, title string, prStatusContext string, masterStatusContext string, prTimeout time.Duration, commitTimeout time.Duration, fn UpdateFunc) error {
	err := g.Clone()
	defer g.Close()
	if err != nil {
		return err
	}

	err = g.PushCommit(branchName, fn)
	if err != nil {
		return err
	}

	err = g.RaisePR(branchName, targetBranch, title, "")
	if err != nil {
		return err
	}

	err = g.WaitForPR(prStatusContext, prTimeout)
	if err != nil {
		return err
	}

	err = g.MergePR()
	if err != nil {
		return err
	}

	return g.WaitForMergeCommit(masterStatusContext, commitTimeout)
}
