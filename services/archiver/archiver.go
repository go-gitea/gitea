// Copyright 2020 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	uri         string
	repoID      int64
	repo        *git.Repository
	refName     string
	ext         string
	archivePath string
	archiveType git.ArchiveType
	commit      *git.Commit
}

// SHA1 hashes will only go up to 40 characters, but SHA256 hashes will go all
// the way to 64.
var shaRegex = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// NewRequest creates an archival request, based on the URI.  The
// resulting ArchiveRequest is suitable for being passed to ArchiveRepository()
// if it's determined that the request still needs to be satisfied.
func NewRequest(repoID int64, repo *git.Repository, uri string) (*ArchiveRequest, error) {
	r := &ArchiveRequest{
		repoID: repoID,
		uri:    uri,
		repo:   repo,
	}

	switch {
	case strings.HasSuffix(uri, ".zip"):
		r.ext = ".zip"
		r.archivePath = path.Join(r.repo.Path, "archives/zip")
		r.archiveType = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		r.ext = ".tar.gz"
		r.archivePath = path.Join(r.repo.Path, "archives/targz")
		r.archiveType = git.TARGZ
	default:
		return nil, fmt.Errorf("Unknown format: %s", uri)
	}

	r.refName = strings.TrimSuffix(r.uri, r.ext)
	var err error

	// Get corresponding commit.
	if r.repo.IsBranchExist(r.refName) {
		r.commit, err = r.repo.GetBranchCommit(r.refName)
		if err != nil {
			return nil, err
		}
	} else if r.repo.IsTagExist(r.refName) {
		r.commit, err = r.repo.GetTagCommit(r.refName)
		if err != nil {
			return nil, err
		}
	} else if shaRegex.MatchString(r.refName) {
		r.commit, err = r.repo.GetCommit(r.refName)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("Unknow ref %s type", r.refName)
	}

	r.archivePath = path.Join(r.archivePath, base.ShortSha(r.commit.ID.String())+r.ext)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// GetArchivePath returns the path from which we can serve this archive.
func (aReq *ArchiveRequest) GetArchivePath() string {
	return aReq.archivePath
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return aReq.refName + aReq.ext
}

func doArchive(r *ArchiveRequest) error {
	ctx, commiter, err := models.TxDBContext()
	if err != nil {
		return err
	}
	defer commiter.Close()

	archiver, err := models.GetRepoArchiver(ctx, r.repoID, r.archiveType, r.commit.ID.String())
	if err != nil {
		return err
	}
	if archiver != nil {
		return nil
	}

	if err := models.AddArchiver(ctx, &models.RepoArchiver{
		RepoID:   r.repoID,
		Type:     r.archiveType,
		CommitID: r.commit.ID.String(),
		Name:     r.GetArchiveName(),
	}); err != nil {
		return err
	}

	rd, w := io.Pipe()
	var done chan error

	go func(done chan error, w io.Writer) {
		err := r.repo.CreateArchive(
			graceful.GetManager().ShutdownContext(),
			r.archiveType,
			w,
			setting.Repository.PrefixArchiveFiles,
			r.commit.ID.String(),
		)
		done <- err
	}(done, w)

	if _, err := storage.RepoArchives.Save(r.archivePath, rd, -1); err != nil {
		return fmt.Errorf("Unable to write archive: %v", err)
	}

	err = <-done
	if err != nil {
		return err
	}

	return commiter.Commit()
}

// ArchiveRepository satisfies the ArchiveRequest being passed in.  Processing
// will occur in a separate goroutine, as this phase may take a while to
// complete.  If the archive already exists, ArchiveRepository will not do
// anything.  In all cases, the caller should be examining the *ArchiveRequest
// being returned for completion, as it may be different than the one they passed
// in.
func ArchiveRepository(request *ArchiveRequest) error {
	return doArchive(request)
}
