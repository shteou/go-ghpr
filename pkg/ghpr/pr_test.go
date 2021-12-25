package ghpr

import (
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func dummyFunc(w *git.Worktree) (string, *object.Signature, error) {
	return "", nil, nil
}

func TestPRUrlNoPrNumber(t *testing.T) {
	repo := newRepo("test", "user", memfs.New(), &mockGoGit{})
	change := NewChange(repo, "test", Credentials{}, dummyFunc)
	pr := newPR(change, nil)

	_, err := pr.URL()
	assert.NotNil(t, err)
}

func TestPRUrl(t *testing.T) {
	repo := newRepo("test", "user", memfs.New(), &mockGoGit{})
	change := NewChange(repo, "test", Credentials{}, dummyFunc)
	pr := newPR(change, nil)
	pr.Number = 1

	url, err := pr.URL()
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/test/user/pulls/1", url)
}
