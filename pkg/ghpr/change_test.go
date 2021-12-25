package ghpr

import (
	"os"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

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

func commitSomething(w *git.Worktree) (string, *object.Signature, error) {
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

func mockRemoteRepository(t *testing.T) (string, *git.Repository) {
	// NOTE: The origin repository will be set when cloned. We're testing
	// the push behaviour, so mock out a local remote
	targetFs, clean := temporalDir()
	defer clean()
	originRepo, err := git.PlainInit(targetFs, true)
	assert.Nil(t, err)
	assert.NotNil(t, originRepo)

	return targetFs, originRepo
}

func TestPushCommit(t *testing.T) {
	// Given a remote repository
	originPath, originRepo := mockRemoteRepository(t)

	// And a cloned repository referencing that remote
	repo, err := initGitRepo()
	assert.Nil(t, err)

	fs := memfs.New()

	r := newRepo("shteou", "go-ghpr", fs, new(mockGoGit))
	r.repo = repo

	_, err = r.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{originPath},
	})
	assert.Nil(t, err)

	// When I make and push the commit
	change := NewChange(r, "foo", Credentials{}, commitSomething)
	err = change.Push()

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

	// Ensure the commit we just made (plus the initial commit for the repo) are pushed
	// to the empty remote repo
	assert.Equal(t, 2, count, "The remote repository had the wrong number of commits")
}
