package ghpr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-github/github"
	"github.com/jpillora/backoff"
	"golang.org/x/oauth2"
)

type PR struct {
	Number    int
	change    Change
	ghClient  *github.Client
	PRSha     string
	MergedSha string
}

func NewPR(change Change, creds Credentials) PR {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: creds.Token},
	)
	tc := oauth2.NewClient(context.TODO(), ts)

	client := github.NewClient(tc)

	return newPR(change, client)
}

func newPR(change Change, client *github.Client) PR {
	return PR{
		change:   change,
		ghClient: client,
	}
}

func (p *PR) Create(targetBranch string, title string, body string) error {
	pr, _, err := p.ghClient.PullRequests.Create(context.TODO(),
		p.change.repo.Org, p.change.repo.Name,
		&github.NewPullRequest{
			Title: &title,
			Head:  &p.change.Branch,
			Base:  &targetBranch,
			Body:  &body})
	if err != nil {
		return err
	}

	p.Number = *pr.Number
	p.PRSha = *pr.Head.SHA

	return err
}

func (p *PR) URL() (string, error) {
	if p.Number == 0 {
		return "", errors.New("Pull Request doesn't have a valid PR number. Was PR creation successful?")
	}

	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", p.change.repo.Org, p.change.repo.Name, p.Number), nil
}

func (p *PR) GetGithubPR() (*github.PullRequest, error) {
	pr, _, err := p.ghClient.PullRequests.Get(context.TODO(), p.change.repo.Org, p.change.repo.Name, p.Number)
	return pr, err
}

func (p *PR) waitForStatus(shaRef string, strategy StatusWaitStrategy, c chan error) {
	b := &backoff.Backoff{
		Min:    strategy.MinPollTime,
		Max:    strategy.MaxPollTime,
		Factor: float64(strategy.PollBackoffFactor),
		Jitter: true,
	}

	for {
		fmt.Printf("Waiting for %s to become mergeable\n", shaRef)
		statuses, _, err := p.ghClient.Repositories.ListStatuses(context.TODO(),
			p.change.repo.Org, p.change.repo.Name,
			shaRef, &github.ListOptions{PerPage: 20})

		if err != nil {
			c <- err
			return
		}

		for i := 0; i < len(statuses); i++ {
			s := statuses[i]
			if s.GetContext() != strategy.WaitStatusContext {
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

func (p *PR) WaitForPRStatus(strategy StatusWaitStrategy) error {
	c := make(chan error, 10)
	go func() {
		p.waitForStatus(p.PRSha, strategy, c)
	}()

	select {
	case err := <-c:
		return err
	case <-time.After(strategy.Timeout):
		return errors.New("timed out waiting for PR to become mergeable")
	}
}

func (p *PR) WaitForMergeStatus(strategy StatusWaitStrategy) error {
	c := make(chan error, 10)
	go func() {
		p.waitForStatus(p.MergedSha, strategy, c)
	}()

	select {
	case err := <-c:
		return err
	case <-time.After(strategy.Timeout):
		return errors.New("timed out waiting for PR to become mergeable")
	}
}

func (p *PR) Merge(mergeMethod string) error {
	pr, err := p.GetGithubPR()
	if err != nil {
		return err
	}

	if pr.Mergeable != nil && *pr.Mergeable {
		merge, _, err := p.ghClient.PullRequests.Merge(context.TODO(),
			p.change.repo.Org, p.change.repo.Name, *pr.Number, "", &github.PullRequestOptions{MergeMethod: mergeMethod})
		if err != nil {
			return err
		}
		p.MergedSha = *merge.SHA
	} else {
		return errors.New("PR is not mergeable")
	}
	return nil
}
