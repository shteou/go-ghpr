# go-ghpr (GitHub PRs)

`go-ghpr` is a simple wrapper around Git and GitHub which helps to automate making changes
to GitHub reositories via Pull Request.


## Features

* Shallow clone a remote repository
* Make a commit to a new branch in the repository, and push that branch to the remote origin
* Cleanup the repository

Planned features:

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
	r, err := ghpr.Clone(repoName, creds)
	if err != nil {
		return err
	}

	defer ghpr.Close(r)

	branchName := fmt.Sprintf("chore-delete-dockerfile-%s", repoName)
	err = ghpr.PushCommit(r, branchName, func(w *git.Worktree) (string, *object.Signature, error) {
		return DeleteDockerfileUpdater(repoName, w)
	})
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := PushDockerfileDeletionBranch("my/repository")
	if err != nil {
		println(err.Error())
	}
}
```
