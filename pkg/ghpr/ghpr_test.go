package ghpr

import (
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage"
	"github.com/stretchr/testify/assert"
)

type mockGoGit struct {
}

func (r mockGoGit) Clone(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error) {
	return &git.Repository{}, nil
}

func TestMakeGithubPR(t *testing.T) {
	// When I create GithubPR instance
	fs := memfs.New()
	pr, err := makeGithubPR(".", Credentials{}, &fs, mockGoGit{})

	// Then there are no errors
	assert.Nil(t, err)
	assert.NotNil(t, pr)
	// And the instance has a filesystem
	assert.NotNil(t, pr.Filesystem)
	// And the instance has a GitHub client
	assert.NotNil(t, pr.GitHubClient)

}

func TestClone(t *testing.T) {
	// Given a GithubPR instance
	fs := memfs.New()
	pr, err := makeGithubPR("shteou/go-ghpr", Credentials{}, &fs, mockGoGit{})
	assert.Nil(t, err)

	// When I perform a clone
	err = pr.Clone()

	// Then there are no errors
	assert.Nil(t, err)
	// And the instance has a Git repository
	assert.NotNil(t, pr.Repo)
	// And the filesystem root is a temporary directory
	assert.Contains(t, pr.Filesystem.Root(), "/repo_")
}
