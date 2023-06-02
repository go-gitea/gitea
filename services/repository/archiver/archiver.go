// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package archiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	RepoID   int64
	refName  string
	Type     git.ArchiveType
	CommitID string
}

// SHA1 hashes will only go up to 40 characters, but SHA256 hashes will go all
// the way to 64.
var shaRegex = regexp.MustCompile(`^[0-9a-f]{4,64}$`)

// ErrUnknownArchiveFormat request archive format is not supported
type ErrUnknownArchiveFormat struct {
	RequestFormat string
}

// Error implements error
func (err ErrUnknownArchiveFormat) Error() string {
	return fmt.Sprintf("unknown format: %s", err.RequestFormat)
}

// Is implements error
func (ErrUnknownArchiveFormat) Is(err error) bool {
	_, ok := err.(ErrUnknownArchiveFormat)
	return ok
}

// RepoRefNotFoundError is returned when a requested reference (commit, tag) was not found.
type RepoRefNotFoundError struct {
	RefName string
}

// Error implements error.
func (e RepoRefNotFoundError) Error() string {
	return fmt.Sprintf("unrecognized repository reference: %s", e.RefName)
}

func (e RepoRefNotFoundError) Is(err error) bool {
	_, ok := err.(RepoRefNotFoundError)
	return ok
}

// NewRequest creates an archival request, based on the URI.  The
// resulting ArchiveRequest is suitable for being passed to ArchiveRepository()
// if it's determined that the request still needs to be satisfied.
func NewRequest(repoID int64, repo *git.Repository, uri string) (*ArchiveRequest, error) {
	r := &ArchiveRequest{
		RepoID: repoID,
	}

	var ext string
	switch {
	case strings.HasSuffix(uri, ".zip"):
		ext = ".zip"
		r.Type = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		ext = ".tar.gz"
		r.Type = git.TARGZ
	case strings.HasSuffix(uri, ".bundle"):
		ext = ".bundle"
		r.Type = git.BUNDLE
	default:
		return nil, ErrUnknownArchiveFormat{RequestFormat: uri}
	}

	r.refName = strings.TrimSuffix(uri, ext)

	var err error
	// Get corresponding commit.
	if repo.IsBranchExist(r.refName) {
		r.CommitID, err = repo.GetBranchCommitID(r.refName)
		if err != nil {
			return nil, err
		}
	} else if repo.IsTagExist(r.refName) {
		r.CommitID, err = repo.GetTagCommitID(r.refName)
		if err != nil {
			return nil, err
		}
	} else if shaRegex.MatchString(r.refName) {
		if repo.IsCommitExist(r.refName) {
			r.CommitID = r.refName
		} else {
			return nil, git.ErrNotExist{
				ID: r.refName,
			}
		}
	} else {
		return nil, RepoRefNotFoundError{RefName: r.refName}
	}

	return r, nil
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return strings.ReplaceAll(aReq.refName, "/", "-") + "." + aReq.Type.String()
}

// Await awaits the completion of an ArchiveRequest. If the archive has
// already been prepared the method returns immediately. Otherwise an archiver
// process will be started and its completion awaited. On success the returned
// RepoArchiver may be used to download the archive. Note that even if the
// context is cancelled/times out a started archiver will still continue to run
// in the background.
func (aReq *ArchiveRequest) Await(ctx context.Context) (*repo_model.RepoArchiver, error) {
	archiver, err := repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		return nil, fmt.Errorf("models.GetRepoArchiver: %w", err)
	}

	if archiver != nil && archiver.Status == repo_model.ArchiverReady {
		// Archive already generated, we're done.
		return archiver, nil
	}

	if err := StartArchive(aReq); err != nil {
		return nil, fmt.Errorf("archiver.StartArchive: %w", err)
	}

	poll := time.NewTicker(time.Second * 1)
	defer poll.Stop()

	for {
		select {
		case <-graceful.GetManager().HammerContext().Done():
			// System stopped.
			return nil, graceful.GetManager().HammerContext().Err()
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-poll.C:
			archiver, err = repo_model.GetRepoArchiver(ctx, aReq.RepoID, aReq.Type, aReq.CommitID)
			if err != nil {
				return nil, fmt.Errorf("repo_model.GetRepoArchiver: %w", err)
			}
			if archiver != nil && archiver.Status == repo_model.ArchiverReady {
				return archiver, nil
			}
		}
	}
}

func doArchive(r *ArchiveRequest) (*repo_model.RepoArchiver, error) {
	txCtx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return nil, err
	}
	defer committer.Close()
	ctx, _, finished := process.GetManager().AddContext(txCtx, fmt.Sprintf("ArchiveRequest[%d]: %s", r.RepoID, r.GetArchiveName()))
	defer finished()

	archiver, err := repo_model.GetRepoArchiver(ctx, r.RepoID, r.Type, r.CommitID)
	if err != nil {
		return nil, err
	}

	if archiver != nil {
		// FIXME: If another process are generating it, we think it's not ready and just return
		// Or we should wait until the archive generated.
		if archiver.Status == repo_model.ArchiverGenerating {
			return nil, nil
		}
	} else {
		archiver = &repo_model.RepoArchiver{
			RepoID:   r.RepoID,
			Type:     r.Type,
			CommitID: r.CommitID,
			Status:   repo_model.ArchiverGenerating,
		}
		if err := repo_model.AddRepoArchiver(ctx, archiver); err != nil {
			return nil, err
		}
	}

	rPath := archiver.RelativePath()
	_, err = storage.RepoArchives.Stat(rPath)
	if err == nil {
		if archiver.Status == repo_model.ArchiverGenerating {
			archiver.Status = repo_model.ArchiverReady
			if err = repo_model.UpdateRepoArchiverStatus(ctx, archiver); err != nil {
				return nil, err
			}
		}
		return archiver, committer.Commit()
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("unable to stat archive: %w", err)
	}

	rd, w := io.Pipe()
	defer func() {
		w.Close()
		rd.Close()
	}()
	done := make(chan error, 1) // Ensure that there is some capacity which will ensure that the goroutine below can always finish
	repo, err := repo_model.GetRepositoryByID(ctx, archiver.RepoID)
	if err != nil {
		return nil, fmt.Errorf("archiver.LoadRepo failed: %w", err)
	}

	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return nil, err
	}
	defer gitRepo.Close()

	go func(done chan error, w *io.PipeWriter, archiver *repo_model.RepoArchiver, gitRepo *git.Repository) {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("%v", r)
			}
		}()

		if archiver.Type == git.BUNDLE {
			err = gitRepo.CreateBundle(
				ctx,
				archiver.CommitID,
				w,
			)
		} else {
			err = gitRepo.CreateArchive(
				ctx,
				archiver.Type,
				w,
				setting.Repository.PrefixArchiveFiles,
				archiver.CommitID,
			)
		}
		_ = w.CloseWithError(err)
		done <- err
	}(done, w, archiver, gitRepo)

	// TODO: add lfs data to zip
	// TODO: add submodule data to zip

	if _, err := storage.RepoArchives.Save(rPath, rd, -1); err != nil {
		return nil, fmt.Errorf("unable to write archive: %w", err)
	}

	err = <-done
	if err != nil {
		return nil, err
	}

	if archiver.Status == repo_model.ArchiverGenerating {
		archiver.Status = repo_model.ArchiverReady
		if err = repo_model.UpdateRepoArchiverStatus(ctx, archiver); err != nil {
			return nil, err
		}
	}

	return archiver, committer.Commit()
}

// ArchiveRepository satisfies the ArchiveRequest being passed in.  Processing
// will occur in a separate goroutine, as this phase may take a while to
// complete.  If the archive already exists, ArchiveRepository will not do
// anything.  In all cases, the caller should be examining the *ArchiveRequest
// being returned for completion, as it may be different than the one they passed
// in.
func ArchiveRepository(request *ArchiveRequest) (*repo_model.RepoArchiver, error) {
	return doArchive(request)
}

var archiverQueue *queue.WorkerPoolQueue[*ArchiveRequest]

// Init initializes archiver
func Init() error {
	handler := func(items ...*ArchiveRequest) []*ArchiveRequest {
		for _, archiveReq := range items {
			log.Trace("ArchiverData Process: %#v", archiveReq)
			if _, err := doArchive(archiveReq); err != nil {
				log.Error("Archive %v failed: %v", archiveReq, err)
			}
		}
		return nil
	}

	archiverQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "repo-archive", handler)
	if archiverQueue == nil {
		return errors.New("unable to create repo-archive queue")
	}
	go graceful.GetManager().RunWithCancel(archiverQueue)

	return nil
}

// StartArchive push the archive request to the queue
func StartArchive(request *ArchiveRequest) error {
	has, err := archiverQueue.Has(request)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	return archiverQueue.Push(request)
}

func deleteOldRepoArchiver(ctx context.Context, archiver *repo_model.RepoArchiver) error {
	if err := repo_model.DeleteRepoArchiver(ctx, archiver); err != nil {
		return err
	}
	p := archiver.RelativePath()
	if err := storage.RepoArchives.Delete(p); err != nil {
		log.Error("delete repo archive file failed: %v", err)
	}
	return nil
}

// DeleteOldRepositoryArchives deletes old repository archives.
func DeleteOldRepositoryArchives(ctx context.Context, olderThan time.Duration) error {
	log.Trace("Doing: ArchiveCleanup")

	for {
		archivers, err := repo_model.FindRepoArchives(repo_model.FindRepoArchiversOption{
			ListOptions: db.ListOptions{
				PageSize: 100,
				Page:     1,
			},
			OlderThan: olderThan,
		})
		if err != nil {
			log.Trace("Error: ArchiveClean: %v", err)
			return err
		}

		for _, archiver := range archivers {
			if err := deleteOldRepoArchiver(ctx, archiver); err != nil {
				return err
			}
		}
		if len(archivers) < 100 {
			break
		}
	}

	log.Trace("Finished: ArchiveCleanup")
	return nil
}

// DeleteRepositoryArchives deletes all repositories' archives.
func DeleteRepositoryArchives(ctx context.Context) error {
	if err := repo_model.DeleteAllRepoArchives(); err != nil {
		return err
	}
	return storage.Clean(storage.RepoArchives)
}
