package ghpr

import (
	"fmt"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/pkg/errors"
)

type Repo struct {
	Name  string
	Owner string
	// The root filesystem in which a temporary filesystem will be created
	rootFilesystem billy.Filesystem
	// the temporary filesystem which houses the repository
	filesystem billy.Filesystem
	git        goGit
	repo       *git.Repository
}

// NewRepo creates a new Repo object with the supplied parameters
func NewRepo(owner string, name string) Repo {
	return newRepo(owner, name, osfs.New("."), realGoGit{})
}

// Clone the remote repository to a temporary directory
func (r *Repo) Clone(creds Credentials) error {
	url := fmt.Sprintf("https://github.com/" + r.Owner + "/" + r.Name)

	auth := http.BasicAuth{Username: creds.Username, Password: creds.Token}

	tempDir, err := util.TempDir(r.rootFilesystem, ".", "repo_")
	if err != nil {
		return errors.Wrap(err, "failed to create temporary directory")
	}

	r.filesystem, err = r.rootFilesystem.Chroot(tempDir)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to change directory to %s", tempDir))
	}

	storageWorkTree, err := r.filesystem.Chroot(".git")
	if err != nil {
		return errors.Wrap(err, "failed to change directory to .git")
	}

	// Pass a defafult LRU object cache, as per git.PlainClone's implementation
	r.repo, err = r.git.Clone(
		filesystem.NewStorage(storageWorkTree, cache.NewObjectLRUDefault()),
		r.filesystem,
		&git.CloneOptions{
			Depth: 1,
			URL:   url,
			Auth:  &auth})

	if err != nil {
		return errors.Wrap(err, "failed to clone remote repository")
	}

	return nil
}

// Close removes the contents of the temporary directory
func (r *Repo) Close() error {
	err := util.RemoveAll(r.filesystem, ".")
	if err != nil {
		return errors.Wrap(err, "failed to clean up temporary directory")
	}
	return nil
}

func newRepo(owner string, name string, fs billy.Filesystem, git goGit) Repo {
	return Repo{
		Name:           name,
		Owner:          owner,
		rootFilesystem: fs,
		filesystem:     nil,
		git:            git,
	}
}
