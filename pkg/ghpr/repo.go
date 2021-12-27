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
	Name           string
	Org            string
	rootFilesystem billy.Filesystem
	filesystem     billy.Filesystem
	git            goGit
	repo           *git.Repository
}

func NewRepo(org string, name string) Repo {
	return newRepo(org, name, osfs.New("."), realGoGit{})
}

func newRepo(org string, name string, fs billy.Filesystem, git goGit) Repo {
	return Repo{
		Name:           name,
		Org:            org,
		rootFilesystem: fs,
		filesystem:     nil,
		git:            git,
	}
}

func (r *Repo) Clone(creds Credentials) error {
	url := fmt.Sprintf("https://github.com/" + r.Org + "/" + r.Name)

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

func (r *Repo) Close() error {
	err := util.RemoveAll(r.filesystem, ".")
	if err != nil {
		return errors.Wrap(err, "failed to clean up temporary directory")
	}
	return nil
}
