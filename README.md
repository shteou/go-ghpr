# go-ghpr (GitHub PRs)

`go-ghpr` is a simple wrapper around Git and GitHub which helps to automate programmatic
changes to GitHub repositories via Pull Requests.

## Supported workflows

`go-ghpr` will make a shallow clone of the remote repository and supports
making a single commit to a local branch. That can then be pushed remotely
to a branch of the same name, and a PR may be raised from that branch to another.

`gh-ghpr` then supports waiting for conditions on that PR (e.g. waiting for it to become
mergeable or to have a passing status check), merging the PR, and again waiting

Most of these steps are optional, so a wide range of workflows can be accomodated.

## Usage

```go
func DeleteDockerfileUpdater(repoName string, w *git.Worktree) (string, *object.Signature, error) {
	w.Remove("Dockerfile")

	commitMessage := "chore: deleting dockerfle for " + repoName
	return commitMessage, &object.Signature{Name: "Stewart Platt", Email: "shteou@gmail.com"}, nil
}

func PushDockerfileDeletionBranch(owner string, name string) error {
	creds := ghpr.Credentials{Username: "***", Token: "***"}
	repo := ghpr.NewRepo(owner, name)
	defer repo.Close()

	err := repo.Clone(creds)
	if err != nil {
		return err
	}

	strategy := ghpr.BackoffStrategy{
		MinPollTime:       time.Second * 20,
		MaxPollTime:       time.Second * 60,
		PollBackoffFactor: 1.05,
	}
	statusChecks := []ghpr.Check{{Name: "Semantic Pull Request", CheckType: "status"}}

	change := ghpr.NewChange(repo, "chore-make-change", creds, func(w *git.Worktree) (string, *object.Signature, error) {
		return UpdateRequirements(service, version, w)
	})

	err = change.Push()
	if err != nil {
		return err
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*30))
	defer cancel()

	pr := ghpr.NewPR(timeout, change, creds)
	pr.Create(timeout, "master", "chore: make change", "")

	err = pr.WaitForPRChecks(timeout, statusChecks, strategy)
	if err != nil {
		return err
	}

	err = pr.Merge(timeout, "merge")
	if err != nil {
		return err
	}

	return pr.WaitForMergeChecks(timeout, statusChecks, strategy)
}

func main() {
	err := PushDockerfileDeletionBranch("my", "repository")
	if err != nil {
		println(err.Error())
	}
}
```
