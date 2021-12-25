package ghpr

import (
	"errors"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func basicMocks() (*mockGoGit, billy.Filesystem) {
	mockGit := new(mockGoGit)
	mockGit.On("Clone",
		mock.MatchedBy(func(s storage.Storer) bool { return true }),
		mock.MatchedBy(func(c *chroot.ChrootHelper) bool { return true }),
		mock.MatchedBy(func(c *git.CloneOptions) bool {
			return c.URL == "https://github.com/shteou/go-ghpr"
		}),
	).Return(&git.Repository{}, nil)
	fs := memfs.New()

	return mockGit, fs
}

func TestRepoCloneDoesntError(t *testing.T) {
	mockGit, fs := basicMocks()

	// Given a repository
	r := newRepo("shteou", "go-ghpr", fs, mockGit)

	// When I clone it
	err := r.Clone(Credentials{})

	// Then there are no errors
	assert.Nil(t, err)
}

func TestRepoCloneIntoDirectory(t *testing.T) {
	mockGit, fs := basicMocks()

	// Given a remote repopsitory
	r := newRepo("shteou", "go-ghpr", fs, mockGit)

	// When I clone the repository
	err := r.Clone(Credentials{})

	// Then there are no errors
	assert.Nil(t, err)
	assert.NotNil(t, r.repo)
	// And the filesystem root is a temporary directory
	assert.Contains(t, r.filesystem.Root(), "/repo_")
}

func TestRepoCloneCloses(t *testing.T) {
	mockGit, fs := basicMocks()
	// Given a cloned repopsitory
	r := newRepo("shteou", "go-ghpr", fs, mockGit)
	_ = r.Clone(Credentials{})
	_, err := r.filesystem.Stat(".")
	assert.Nil(t, err)

	// When I close the repository
	err = r.Close()

	// Then there are no errors
	assert.Nil(t, err)

	// And the directory no longer exists
	_, err = r.filesystem.Stat(".")
	assert.NotNil(t, err)
}

func TestCloneFailure(t *testing.T) {
	mockGit := new(mockGoGit)
	mockGit.On("Clone",
		mock.MatchedBy(func(s storage.Storer) bool { return true }),
		mock.MatchedBy(func(c *chroot.ChrootHelper) bool { return true }),
		mock.MatchedBy(func(c *git.CloneOptions) bool {
			return c.URL == "https://github.com/shteou/invalid"
		}),
	).Return(nil, errors.New("fail1"))
	fs := memfs.New()

	// Given a repository
	r := newRepo("shteou", "invalid", fs, mockGit)

	// When I perform a clone
	err := r.Clone(Credentials{})

	// Then an error is returned
	assert.NotNil(t, err)
}
