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
	change, err := ghpr.MakeGithubPR("your/repository", creds)
	if err != nil {
		return err
	}

	return change.Create("chore-make-change", "master", "Semantic Pull Request", "Semantic Pull Request", func(w *git.Worktree) (string, *object.Signature, error) {
		return UpdateRequirements(service, version, w)
	})
}

func main() {
	err := PushDockerfileDeletionBranch("my/repository")
	if err != nil {
		println(err.Error())
	}
}
```
