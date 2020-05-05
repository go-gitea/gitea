// Copyright 2020 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package archiver

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/unknwon/com"
)

// ArchiveRequest defines the parameters of an archive request, which notably
// includes the specific repository being archived as well as the commit, the
// name by which it was requested, and the kind of archive being requested.
// This is entirely opaque to external entities, though, and mostly used as a
// handle elsewhere.
type ArchiveRequest struct {
	uri             string
	repo            *git.Repository
	refName         string
	ext             string
	archivePath     string
	archiveType     git.ArchiveType
	archiveComplete bool
	commit          *git.Commit
}

var archiveInProgress []*ArchiveRequest
var archiveMutex sync.Mutex

// These facilitate testing, by allowing the unit tests to control (to some extent)
// the goroutine used for processing the queue.
var archiveQueueMutex *sync.Mutex
var archiveQueueStartCond *sync.Cond
var archiveQueueReleaseCond *sync.Cond

// GetArchivePath returns the path from which we can serve this archive.
func (aReq *ArchiveRequest) GetArchivePath() string {
	return aReq.archivePath
}

// GetArchiveName returns the name of the caller, based on the ref used by the
// caller to create this request.
func (aReq *ArchiveRequest) GetArchiveName() string {
	return aReq.refName + aReq.ext
}

// IsComplete returns the completion status of this request.
func (aReq *ArchiveRequest) IsComplete() bool {
	return aReq.archiveComplete
}

// The caller must hold the archiveMutex across calls to getArchiveRequest.
func getArchiveRequest(repo *git.Repository, commit *git.Commit, archiveType git.ArchiveType) *ArchiveRequest {
	for _, r := range archiveInProgress {
		// Need to be referring to the same repository.
		if r.repo.Path == repo.Path && r.commit.ID == commit.ID && r.archiveType == archiveType {
			return r
		}
	}
	return nil
}

// DeriveRequestFrom creates an archival request, based on the URI.  The
// resulting ArchiveRequest is suitable for being passed to ArchiveRepository()
// if it's determined that the request still needs to be satisfied.
func DeriveRequestFrom(ctx *context.Context, uri string) *ArchiveRequest {
	if ctx.Repo == nil || ctx.Repo.GitRepo == nil {
		log.Trace("Repo not initialized")
		ctx.Error(404)
		return nil
	}
	r := &ArchiveRequest{
		uri:  uri,
		repo: ctx.Repo.GitRepo,
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
		log.Trace("Unknown format: %s", uri)
		ctx.Error(404)
		return nil
	}

	r.refName = strings.TrimSuffix(r.uri, r.ext)
	if !com.IsDir(r.archivePath) {
		if err := os.MkdirAll(r.archivePath, os.ModePerm); err != nil {
			ctx.ServerError("Download -> os.MkdirAll(archivePath)", err)
			return nil
		}
	}

	// Get corresponding commit.
	var (
		err error
	)
	if r.repo.IsBranchExist(r.refName) {
		r.commit, err = r.repo.GetBranchCommit(r.refName)
		if err != nil {
			ctx.ServerError("GetBranchCommit", err)
			return nil
		}
	} else if r.repo.IsTagExist(r.refName) {
		r.commit, err = r.repo.GetTagCommit(r.refName)
		if err != nil {
			ctx.ServerError("GetTagCommit", err)
			return nil
		}
	} else if len(r.refName) >= 4 && len(r.refName) <= 40 {
		r.commit, err = r.repo.GetCommit(r.refName)
		if err != nil {
			ctx.NotFound("GetCommit", nil)
			return nil
		}
	} else {
		ctx.NotFound("DeriveRequestFrom", nil)
		return nil
	}

	archiveMutex.Lock()
	defer archiveMutex.Unlock()
	if rExisting := getArchiveRequest(r.repo, r.commit, r.archiveType); rExisting != nil {
		return rExisting
	}

	r.archivePath = path.Join(r.archivePath, base.ShortSha(r.commit.ID.String())+r.ext)
	r.archiveComplete = com.IsFile(r.archivePath)
	return r
}

func doArchive(r *ArchiveRequest) {
	var (
		err         error
		tmpArchive  *os.File
		destArchive *os.File
	)

	// It could have happened that we enqueued two archival requests, due to
	// race conditions and difficulties in locking.  Do one last check that
	// the archive we're referring to doesn't already exist.  If it does exist,
	// then just mark the request as complete and move on.
	if com.IsFile(r.archivePath) {
		r.archiveComplete = true
		return
	}

	// Create a temporary file to use while the archive is being built.  We
	// will then copy it into place (r.archivePath) once it's fully
	// constructed.
	tmpArchive, err = ioutil.TempFile("", "archive")
	if err != nil {
		log.Error("Unable to create a temporary archive file! Error: %v", err)
		return
	}
	defer func() {
		tmpArchive.Close()
		os.Remove(tmpArchive.Name())
	}()

	if err = r.commit.CreateArchive(tmpArchive.Name(), git.CreateArchiveOpts{
		Format: r.archiveType,
		Prefix: setting.Repository.PrefixArchiveFiles,
	}); err != nil {
		log.Error("Download -> CreateArchive "+tmpArchive.Name(), err)
		return
	}

	// Now we copy it into place
	if destArchive, err = os.Create(r.archivePath); err != nil {
		log.Error("Unable to open archive " + r.archivePath)
		return
	}
	_, err = io.Copy(destArchive, tmpArchive)
	destArchive.Close()
	if err != nil {
		log.Error("Unable to write archive " + r.archivePath)
		return
	}

	r.archiveComplete = true
}

// ArchiveRepository satisfies the ArchiveRequest being passed in.  Processing
// will occur in a separate goroutine, as this phase may take a while to
// complete.  If the archive already exists, ArchiveRepository will not do
// anything.
func ArchiveRepository(request *ArchiveRequest) {
	if request.archiveComplete {
		return
	}
	go func() {
		// We'll take some liberties here, in that the caller may not assume that the
		// specific request they submitted is the one getting enqueued.  We'll just drop
		// it if it turns out we've already enqueued an identical request, as they'll keep
		// checking back for the status anyways.
		archiveMutex.Lock()
		if rExisting := getArchiveRequest(request.repo, request.commit, request.archiveType); rExisting != nil {
			archiveMutex.Unlock()
			return
		}
		archiveInProgress = append(archiveInProgress, request)
		archiveMutex.Unlock()

		// Wait to start, if we have the Cond for it.  This is currently only
		// useful for testing, so that the start and release of queued entries
		// can be controlled to examine the queue.
		if archiveQueueStartCond != nil {
			archiveQueueMutex.Lock()
			archiveQueueStartCond.Wait()
			archiveQueueMutex.Unlock()
		}

		// Drop the mutex while we process the request.  This may take a long
		// time, and it's not necessary now that we've added the reequest to
		// archiveInProgress.
		doArchive(request)

		if archiveQueueReleaseCond != nil {
			archiveQueueMutex.Lock()
			archiveQueueReleaseCond.Wait()
			archiveQueueMutex.Unlock()
		}

		// Purge this request from the list.  To do so, we'll just take the
		// index at which we ended up at and swap the final element into that
		// position, then chop off the now-redundant final element.  The slice
		// may have change in between these two segments and we may have moved,
		// so we search for it here.  We could perhaps avoid this search
		// entirely if len(archiveInProgress) == 1, but we should verify
		// correctness.
		archiveMutex.Lock()
		defer archiveMutex.Unlock()
		idx := -1
		for _idx, req := range archiveInProgress {
			if req == request {
				idx = _idx
				break
			}
		}
		if idx == -1 {
			log.Error("ArchiveRepository: Failed to find request for removal.")
			return
		}
		lastidx := len(archiveInProgress) - 1
		if idx != lastidx {
			archiveInProgress[idx] = archiveInProgress[lastidx]
		}
		archiveInProgress = archiveInProgress[:lastidx]
	}()
}
