package ghpr

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/github"
	"github.com/jpillora/backoff"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type PR struct {
	Number    int
	change    Change
	ghClient  *github.Client
	PRSha     string
	MergedSha string
}

// NewPR creates a new PR object. The supplied context may be used
// over the course of the PR object's lifetime
func NewPR(ctx context.Context, change Change, creds Credentials) PR {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: creds.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return newPR(change, client)
}

// Create a PR in Github from the Change's source branch to the supplied target branch
func (p *PR) Create(ctx context.Context, targetBranch string, title string, body string) error {
	pr, _, err := p.ghClient.PullRequests.Create(ctx,
		p.change.repo.Owner, p.change.repo.Name,
		&github.NewPullRequest{
			Title: &title,
			Head:  &p.change.Branch,
			Base:  &targetBranch,
			Body:  &body})
	if err != nil {
		return errors.Wrap(err, "failed to create PR")
	}

	p.Number = *pr.Number
	p.PRSha = *pr.Head.SHA

	return nil
}

// GetGithubPR feches the latest Github PR object directly
func (p *PR) GetGithubPR(ctx context.Context) (*github.PullRequest, error) {
	pr, _, err := p.ghClient.PullRequests.Get(ctx, p.change.repo.Owner, p.change.repo.Name, p.Number)
	return pr, err
}

// Merge the PR using the supplied mergeMethod (one of merge, rebase or squash).
func (p *PR) Merge(ctx context.Context, mergeMethod string) error {
	pr, err := p.GetGithubPR(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve GitHub PR")
	}

	if pr.Mergeable != nil && *pr.Mergeable {
		merge, _, err := p.ghClient.PullRequests.Merge(ctx,
			p.change.repo.Owner, p.change.repo.Name, *pr.Number, "", &github.PullRequestOptions{MergeMethod: mergeMethod})
		if err != nil {
			return errors.Wrap(err, "failedd to merge PR")
		}
		p.MergedSha = *merge.SHA
	} else {
		return errors.New("PR is not mergeable")
	}
	return nil
}

// WaitForPRChecks polls for GitHub action/status results on a given PR (the HEAD of the source branch)
// with exponential backoff
func (p *PR) WaitForPRChecks(ctx context.Context, checks []Check, backoffStrategy BackoffStrategy) error {
	return p.waitForChecks(ctx, p.PRSha, checks, backoffStrategy)
}

// WaitForMergeChecks polls for GitHub action/status results on the merged commit of a PR (a reference on
// the target branch) with exponential backoff
func (p *PR) WaitForMergeChecks(ctx context.Context, checks []Check, backoffStrategy BackoffStrategy) error {
	return p.waitForChecks(ctx, p.MergedSha, checks, backoffStrategy)
}

// URL fetches the URL for the GitHub PR without any additional calls to GitHub
// The function returns an error if the PR has not yet been created
func (p *PR) URL() (string, error) {
	if p.Number == 0 {
		return "", errors.New("pull request doesn't have a valid PR number (was PR creation successful?)")
	}

	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", p.change.repo.Owner, p.change.repo.Name, p.Number), nil
}

func newPR(change Change, client *github.Client) PR {
	return PR{
		change:   change,
		ghClient: client,
	}
}

func statusSuccessful(targetStatus string, statuses []*github.RepoStatus) (string, error) {
	for _, status := range statuses {
		context := status.GetContext()

		if context != targetStatus {
			continue
		}

		return status.GetState(), nil
	}

	return "", errors.New("Could not find target context in commit status list")
}

func (p *PR) waitForChecks(ctx context.Context, shaRef string, checks []Check, backoffStrategy BackoffStrategy) error {
	b := &backoff.Backoff{
		Min:    backoffStrategy.MinPollTime,
		Max:    backoffStrategy.MaxPollTime,
		Factor: float64(backoffStrategy.PollBackoffFactor),
		Jitter: true,
	}

	targetStatuses := []string{}
	targetActions := []string{}
	for _, check := range checks {
		if check.CheckType == "status" {
			targetStatuses = append(targetStatuses, check.Name)
		} else if check.CheckType == "action" {
			targetActions = append(targetActions, check.Name)
		} else {
			return errors.New("Unknown check type, must be one of 'action' or 'status'")
		}
	}

	if len(targetActions) > 0 {
		return errors.New("Unsupported check type, check not yet implemented")
	}

	for {
		statuses, _, err := p.ghClient.Repositories.ListStatuses(ctx,
			p.change.repo.Owner, p.change.repo.Name,
			shaRef, &github.ListOptions{PerPage: 20})

		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed listing statuses while waiting for %s", shaRef))
		}

		statusesSuccessful := 0
		for _, status := range targetStatuses {
			result, err := statusSuccessful(status, statuses)
			if err != nil {
				// If a status is not found yet, wait for next poll
				break
			}

			if result == "success" {
				statusesSuccessful += 1
				continue
			} else if result == "failure" || result == "error" {
				return fmt.Errorf("target status check (%s) is in a failed state, aborting", status)
			}
		}

		if statusesSuccessful == len(targetStatuses) {
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("timed out waiting for status")
		case <-time.After(b.Duration()):
		}
	}
}
