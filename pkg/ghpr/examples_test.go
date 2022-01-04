package ghpr_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shteou/go-ghpr/pkg/ghpr"
)

func basicChange() (*ghpr.Change, error) {
	creds := ghpr.Credentials{Username: "shteou", Token: "test"}

	repo := ghpr.NewRepo("shteou", "go-ghpr")
	defer repo.Close()

	err := repo.Clone(creds)
	if err != nil {
		return nil, err
	}

	change := ghpr.NewChange(repo, "test-branch", creds, updater)
	err = change.Push()
	if err != nil {
		return nil, err
	}

	return &change, nil
}

func basicPR() (*ghpr.PR, error) {
	change, err := basicChange()
	if err != nil {
		return nil, err
	}

	pr := ghpr.NewPR(context.Background(), *change, creds())
	return &pr, nil
}

func creds() ghpr.Credentials {
	return ghpr.Credentials{Username: "shteou", Token: "test"}
}

func ExampleRepo_Clone() {
	repo := ghpr.NewRepo("shteou", "go-ghpr")
	defer repo.Close()

	_ = repo.Clone(ghpr.Credentials{Username: "shteou", Token: "test"})
}

func ExampleNewPR() {
	change, _ := basicChange()

	_ = ghpr.NewPR(context.Background(), *change, creds())
}

func ExamplePR_Create() {
	basicChange, _ := basicChange()
	pr := ghpr.NewPR(context.Background(), *basicChange, creds())
	_ = pr.Create(context.Background(), "main", "chore: remove obsolete files", "")

	url, _ := pr.URL()
	fmt.Printf("New pull request raised at %s\n", url)
}

func ExamplePR_WaitForPRStatus() {
	pr, _ := basicPR()
	_ = pr.Create(context.Background(), "main", "chore: remove obsolete files", "")

	strategy := ghpr.BackoffStrategy{
		MinPollTime:       10 * time.Second,
		MaxPollTime:       60 * time.Second,
		PollBackoffFactor: 1.05,
	}
	statusChecks := []ghpr.Check{{Name: "Semantic Pull Request", CheckType: "status"}}

	_ = pr.WaitForPRChecks(context.Background(), statusChecks, strategy)
}

func ExamplePR_WaitForMergeable() {
	pr, _ := basicPR()
	_ = pr.Create(context.Background(), "main", "chore: remove obsolete files", "")
	strategy := ghpr.BackoffStrategy{MinPollTime: 10 * time.Second, MaxPollTime: 60 * time.Second, PollBackoffFactor: 1.05}
	pr.WaitForPRMergeable(context.Background(), strategy)
}

func ExamplePR_Merge() {
	pr, _ := basicPR()
	_ = pr.Create(context.Background(), "main", "chore: remove obsolete files", "")
	_ = pr.Merge(context.Background(), "squash")
}

func ExamplePR_WaitForMergeStatus() {
	pr, _ := basicPR()
	_ = pr.Create(context.Background(), "main", "chore: remove obsolete files", "")

	strategy := ghpr.BackoffStrategy{MinPollTime: 10 * time.Second, MaxPollTime: 60 * time.Second, PollBackoffFactor: 1.05}
	statusChecks := []ghpr.Check{{Name: "Semantic Pull Request", CheckType: "status"}}
	_ = pr.WaitForPRChecks(context.Background(), statusChecks, strategy)
	_ = pr.Merge(context.Background(), "squash")

	_ = pr.WaitForMergeChecks(context.Background(), statusChecks, strategy)
}
