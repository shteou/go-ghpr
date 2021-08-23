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

// Testify!!
type mockGoGit struct {
	mock.Mock
}

func (g *mockGoGit) Clone(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error) {
	args := g.Called(s, worktree, o)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*git.Repository), args.Error(1)
}

func TestMakeGithubPR(t *testing.T) {
	mockGit := new(mockGoGit)

	// When I create GithubPR instance
	fs := memfs.New()
	pr, err := makeGithubPR("shteou/go-ghpr", Credentials{}, &fs, mockGit)

	// Then there are no errors
	assert.Nil(t, err)
	assert.NotNil(t, pr)
	// And the instance has a filesystem
	assert.NotNil(t, pr.filesystem)
	// And the instance has a GitHub client
	assert.NotNil(t, pr.gitHubClient)
}

func TestMakeGithubPRWithInvalidRepo(t *testing.T) {
	mockGit := new(mockGoGit)

	// When I create GithubPR instance with a missing user or repository
	fs := memfs.New()
	pr, err := makeGithubPR("shteou", Credentials{}, &fs, mockGit)

	// Then there is one error
	assert.NotNil(t, err)
	// And no GithubPR instance is returned
	assert.Nil(t, pr)
}

func TestMakeGithubPRWithEmptyRepo(t *testing.T) {
	// When I create GithubPR instance with an empty user/repository
	fs := memfs.New()
	pr, err := makeGithubPR("", Credentials{}, &fs, &mockGoGit{})

	// Then there is one error
	assert.NotNil(t, err)
	// And no GithubPR instance is returned
	assert.Nil(t, pr)
}

func TestMakeGithubPRWithTooManySlashes(t *testing.T) {
	// When I create GithubPR instance with extra slashes
	fs := memfs.New()
	pr, err := makeGithubPR("shteou/go-ghpr/foo", Credentials{}, &fs, &mockGoGit{})

	// Then there is one error
	assert.NotNil(t, err)
	// And no GithubPR instance is returned
	assert.Nil(t, pr)
}

func TestClone(t *testing.T) {
	// Given a GithubPR instance
	mockGit := new(mockGoGit)
	mockGit.On("Clone",
		mock.MatchedBy(func(s storage.Storer) bool { return true }),
		mock.MatchedBy(func(c *chroot.ChrootHelper) bool { return true }),
		mock.MatchedBy(func(c *git.CloneOptions) bool {
			return c.URL == "https://github.com/shteou/go-ghpr"
		}),
	).Return(&git.Repository{}, nil)

	fs := memfs.New()
	pr, err := makeGithubPR("shteou/go-ghpr", Credentials{}, &fs, mockGit)
	assert.Nil(t, err)



	// When I perform a clone
	err = pr.Clone()

	// Then there are no errors
	assert.Nil(t, err)
	// And the instance has a Git repository
	assert.NotNil(t, pr.gitRepo)
	// And the filesystem root is a temporary directory
	assert.Contains(t, pr.filesystem.Root(), "/repo_")
}


func TestCloneFailure(t *testing.T) {
	// Given a GithubPR instance
	mockGit := new(mockGoGit)
	mockGit.On("Clone",
		mock.MatchedBy(func(s storage.Storer) bool { return true }),
		mock.MatchedBy(func(c *chroot.ChrootHelper) bool { return true }),
		mock.MatchedBy(func(c *git.CloneOptions) bool {
			return c.URL == "https://github.com/shteou/invalid"
		}),
	).Return(nil, errors.New("fail1"))

	fs := memfs.New()
	pr, err := makeGithubPR("shteou/invalid", Credentials{}, &fs, mockGit)
	assert.Nil(t, err)
	assert.NotNil(t, pr)

	// When I perform a clone
	err = pr.Clone()

	// Then there are no errors
	assert.NotNil(t, err)
}
