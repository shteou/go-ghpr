package ghpr_test

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/shteou/go-ghpr/pkg/ghpr"
)

func updater(w *git.Worktree) (string, *object.Signature, error) {
	_, err := w.Remove("test-file")
	if err != nil {
		return "", nil, err
	}

	return "chore: remove obsolete test-file", &object.Signature{Name: "Stew", Email: "shteou@gmail.com"}, nil
}

func ExampleChange_Push() {
	creds := ghpr.Credentials{Username: "shteou", Token: "test"}

	repo := ghpr.NewRepo("shteou", "go-ghpr")
	defer repo.Close()

	err := repo.Clone(creds)
	if err != nil {
		return
	}

	change := ghpr.NewChange(repo, "test-branch", creds, updater)
	err = change.Push()
}
