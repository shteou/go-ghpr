# go-ghpr (GitHub PRs)

`go-ghpr` is a simple wrapper around Git and GitHub which helps to automate making changes
to GitHub reositories via Pull Request.


## Features

* Shallow clone a remote repository
* Make a commit to a new branch in the repository, and push that branch to the remote origin
* Cleanup the repository
* Raise a PR for a source/target branch
* Wait for the PR to become mergeable and merge it
* Wait for a status on the merged commit

## Usage

```go
func DeleteDockerfileUpdater(repoName string, w *git.Worktree) (string, *object.Signature, error) {
	w.Remove("Dockerfile")

	commitMessage := "chore: deleting dockerfle for " + repoName
	return commitMessage, &object.Signature{Name: "Stewart Platt", Email: "shteou@gmail.com"}, nil
}

func PushDockerfileDeletionBranch(repoName string) error {
	creds := ghpr.Credentials{Username: "***", Token: "***"}
	change := ghpr.MakeGithubPR("your/repository", creds, nil)

	err := change.Clone()
	if err != nil {
		return err
	}

	defer change.Close()

	branchName := fmt.Sprintf("chore-%s-%s", service, version)
	err = change.PushCommit(branchName, func(w *git.Worktree) (string, *object.Signature, error) {
		return UpdateRequirements(service, version, w)
	})
	if err != nil {
		return err
	}

	title := fmt.Sprintf("chore: remove dockerfile for %s", service)
	err = change.RaisePR(branchName, "master", title, """)
	if err != nil {
		return err
	}

	// Wait for the PR's Semantic Pull Request status to become successful
	err = change.WaitForPR("Semantic Pull Request")
	if err != nil {
		return err
	}

	err = change.MergePR()
	if err != nil {
		return err
	}

	return change.WaitForMergeCommit("Semantic Pull Request")
}

func main() {
	err := PushDockerfileDeletionBranch("my/repository")
	if err != nil {
		println(err.Error())
	}
}
```
