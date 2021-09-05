package ghpr

import (
	"errors"
	"os"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockGoGit is a partial mock of go-git
// It mocks out the externally interfacing methods (e.g. clone/push)
// but forwards the others to go-git where it can operate on the repository
// in memory
type mockGoGit struct {
	mock.Mock
}

// Use testify
func (g *mockGoGit) Clone(s storage.Storer, worktree billy.Filesystem, o *git.CloneOptions) (*git.Repository, error) {
	args := g.Called(s, worktree, o)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*git.Repository), args.Error(1)
}

func (g *mockGoGit) Push(o *git.PushOptions) error {

	return nil
}

// Initialises a basic git repository, makes a minimal initial commit
// and sets the origin remote. This emulates a typical cloned repository
func initGitRepo() (*git.Repository, error) {
	fs := memfs.New()
	storer := memory.NewStorage()

	repository, err := git.Init(storer, fs)
	if err != nil {
		return nil, err
	}

	wt, err := repository.Worktree()
	if err != nil {
		return nil, err
	}
	wtFs := wt.Filesystem

	_, err = wtFs.Create("test")
	if err != nil {
		return nil, err
	}

	_, err = wt.Add("test")
	if err != nil {
		return nil, err
	}

	wt.Commit("first commit!", &git.CommitOptions{Author: &object.Signature{Name: "test", Email: "test.test"}})

	return repository, nil
}

func setupMockGoGit(t *testing.T) *mockGoGit {
	gitRepo, err := initGitRepo()
	assert.Nil(t, err)

	mockGit := new(mockGoGit)
	mockGit.On("Clone",
		mock.MatchedBy(func(s storage.Storer) bool { return true }),
		mock.MatchedBy(func(c *chroot.ChrootHelper) bool { return true }),
		mock.MatchedBy(func(c *git.CloneOptions) bool {
			return c.URL == "https://github.com/shteou/go-ghpr"
		}),
	).Return(gitRepo, nil)

	return mockGit
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

func commitNothing(w *git.Worktree) (string, *object.Signature, error) {
	f, _ := w.Filesystem.Create("test_file")
	_, _ = f.Write([]byte("My data"))
	_ = f.Close()
	w.Add("test_file")
	return "committed something!", &object.Signature{Name: "author", Email: "test@currencycloud.com"}, nil
}

func temporalDir() (path string, clean func()) {
	fs := osfs.New(os.TempDir())
	path, err := util.TempDir(fs, "", "")
	if err != nil {
		panic(err)
	}

	return fs.Join(fs.Root(), path), func() {
		util.RemoveAll(fs, path)
	}
}

func TestPushCommit(t *testing.T) {
	// Given a cloned repository
	repo, err := initGitRepo()
	assert.Nil(t, err)

	fs := memfs.New()
	pr, err := makeGithubPR("shteou/go-ghpr", Credentials{}, &fs, new(mockGoGit))
	assert.Nil(t, err)
	pr.gitRepo = repo

	// And a remote repository
	// NOTE: The origin repository will be set when cloned. We're testing
	// the push behaviour, so mock out a local remote
	targetFs, clean := temporalDir()
	defer clean()
	originRepo, err := git.PlainInit(targetFs, true)
	assert.Nil(t, err)
	assert.NotNil(t, originRepo)

	_, err = pr.gitRepo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{targetFs},
	})
	assert.Nil(t, err)

	// When I make and push the commit
	err = pr.PushCommit("my-branch", commitNothing)

	// Then there are no errors
	assert.Nil(t, err)

	// And the commit has been pushed to the remote repository
	commitIter, err := originRepo.CommitObjects()
	assert.Nil(t, err)

	count := 0
	commitIter.ForEach(func(c *object.Commit) error {
		count += 1
		return nil
	})

	assert.Equal(t, 2, count, "The remote repository had the wrong number of commits")
}
