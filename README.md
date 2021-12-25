# go-ghpr (GitHub PRs)

`go-ghpr` is a simple wrapper around Git and GitHub which helps to automate making changes
to GitHub repositories via Pull Request.


## Features

* Shallow clone a remote repository
* Make a commit to a new branch in the repository, and push that branch to the remote origin
* Raise a PR for a source/target branch
* Wait for the PR to become mergeable and merge it
* Wait for a status on the merged commit
* Cleanup the repository

### Planned features

* A more extensible API which still abstracts some Git/GitHub plumbing  
  The current API is very tailored to a single use case of making a single change,
  raising a PR, and merging that PR
* Add more strategies for waiting for on PR status  
  e.g. waiting for classic status checks and those from GitHub actions or
  waiting on all status checks / mergeable status
* Support for different merge strategies (and identifying which commits to
  wait for statuses on)

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

	strategy := ghpr.StatusWaitStrategy{
		MinPollTime:       time.Second * 20,
		MaxPollTime:       time.Second * 60,
		PollBackoffFactor: 1.05,
		Timeout:           time.Minute * 10,
		WaitStatusContext: "Semantic Pull Request",
	}

	change := ghpr.NewChange(repo, "chore-make-change", creds, func(w *git.Worktree) (string, *object.Signature, error) {
		return UpdateRequirements(service, version, w)
	})

	err = change.Push()
	if err != nil {
		return err
	}

	pr := ghpr.NewPR(change, creds)
	pr.Create("master", "chore: make change", "")

	err = pr.WaitForPRStatus(strategy)
	if err != nil {
		return err
	}

	err = pr.Merge("merge")
	if err != nil {
		return err
	}

	return pr.WaitForMergeStatus(strategy)
}

func main() {
	err := PushDockerfileDeletionBranch("my", "repository")
	if err != nil {
		println(err.Error())
	}
}
```
