package ghpr

import (
	"github.com/go-git/go-billy/v5"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage"
	"github.com/stretchr/testify/mock"
)

// mockGoGit is a partial mock of go-git
// It mocks out the externally interfacing methods (e.g. clone), but
// forwards the others to go-git where it can operate on the repository
// in memory
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

func (g *mockGoGit) Push(o *git.PushOptions) error {
	return nil
}
