// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	git_model "gitea.dev/models/git"
	lfs_module "gitea.dev/modules/lfs"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	files_service "gitea.dev/services/repository/files"
)

// A blob at or under this size is buffered so it can be checked for already
// being a pointer. Anything larger cannot be one.
const maxPointerSize = 1024

type promoteEntry struct {
	pointer   lfs_module.Pointer
	exists    bool
	filePaths []string
}

// PromoteToLFS moves files already committed as git blobs into Git LFS.
//
// This exists because it cannot be done from outside. Over the contents API a
// committed pointer is read as content and re-encoded into a pointer to a
// pointer, and re-committing a file's own bytes is identical content, so no
// commit is made and the LFS filter never runs. Both were measured against a
// running server before this endpoint was written.
//
// Here the bytes never leave Gitea: each blob is read from the object store it
// already sits in and streamed into the LFS store, so there is no request body
// and no size ceiling. An ordinary commit then replaces each file with its
// pointer. History is not rewritten -- the original blobs stay where they are,
// so the repository does not shrink, and this commit reverts like any other.
func PromoteToLFS(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/lfs/promote repository repoPromoteToLFS
	// ---
	// summary: Move files already committed as git blobs into Git LFS
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PromoteToLFSOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PromoteToLFSResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	if !setting.LFS.StartServer {
		ctx.APIError(http.StatusUnprocessableEntity, "LFS is not enabled on this server")
		return
	}
	opts := web.GetForm(ctx).(*api.PromoteToLFSOptions)
	if len(opts.Files) == 0 {
		ctx.APIError(http.StatusUnprocessableEntity, "no files given")
		return
	}

	branch := opts.Branch
	if branch == "" {
		branch = ctx.Repo.Repository.DefaultBranch
	}
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(branch)
	if err != nil {
		ctx.APIErrorNotFound("branch")
		return
	}

	contentStore := lfs_module.NewContentStore()
	entriesByOID := map[string]*promoteEntry{}

	// Pass one reads and hashes, and writes nothing. A promote that turns out
	// to be refused must not have left objects in the store behind it.
	for _, filePath := range deduplicate(opts.Files) {
		blob, err := commit.GetBlobByPath(filePath)
		if err != nil {
			ctx.APIErrorNotFound("file")
			return
		}
		dataRc, err := blob.DataAsync()
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		var pointer lfs_module.Pointer
		if blob.Size() <= maxPointerSize {
			content, readErr := io.ReadAll(dataRc)
			_ = dataRc.Close()
			if readErr != nil {
				ctx.APIErrorInternal(readErr)
				return
			}
			if _, pErr := lfs_module.ReadPointerFromBuffer(content); pErr == nil {
				continue // already a pointer: nothing to move
			}
			pointer, err = lfs_module.GeneratePointer(bytes.NewReader(content))
		} else {
			// Streamed, not buffered: these are the files worth promoting and
			// they are routinely gigabytes.
			pointer, err = lfs_module.GeneratePointer(dataRc)
			_ = dataRc.Close()
		}
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}

		if seen, ok := entriesByOID[pointer.Oid]; ok {
			// Two paths holding identical bytes are one object.
			seen.filePaths = append(seen.filePaths, filePath)
			continue
		}
		exists, err := contentStore.Exists(pointer)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		entriesByOID[pointer.Oid] = &promoteEntry{pointer: pointer, exists: exists, filePaths: []string{filePath}}
	}

	if len(entriesByOID) == 0 {
		ctx.JSON(http.StatusOK, &api.PromoteToLFSResponse{Moved: []string{}})
		return
	}

	// Branch protection is checked before any durable write, for the same
	// reason pass one writes nothing: a rejected commit must not leave objects
	// in the store with no tree pointing at them.
	var treePaths []string
	for _, entry := range entriesByOID {
		treePaths = append(treePaths, entry.filePaths...)
	}
	if err := files_service.VerifyBranchProtection(
		ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.Doer, branch, treePaths,
	); err != nil {
		ctx.APIError(http.StatusForbidden, err.Error())
		return
	}

	var changes []*files_service.ChangeRepoFile
	for _, entry := range entriesByOID {
		if !entry.exists {
			// Reopened: pass one exhausted the reader hashing it.
			srcBlob, blobErr := commit.GetBlobByPath(entry.filePaths[0])
			if blobErr != nil {
				ctx.APIErrorInternal(blobErr)
				return
			}
			dataRc, blobErr := srcBlob.DataAsync()
			if blobErr != nil {
				ctx.APIErrorInternal(blobErr)
				return
			}
			putErr := contentStore.Put(entry.pointer, dataRc)
			_ = dataRc.Close()
			if putErr != nil {
				ctx.APIErrorInternal(putErr)
				return
			}
		}
		if _, err := git_model.NewLFSMetaObject(ctx, ctx.Repo.Repository.ID, entry.pointer); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		for _, filePath := range entry.filePaths {
			changes = append(changes, &files_service.ChangeRepoFile{
				Operation:     "update",
				TreePath:      filePath,
				ContentReader: strings.NewReader(entry.pointer.StringContent()),
				IsLFSPointer:  true,
			})
		}
	}

	message := opts.Message
	if message == "" {
		message = "Store large files in Git LFS"
	}
	if _, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, &files_service.ChangeRepoFilesOptions{
		OldBranch: branch,
		NewBranch: branch,
		Message:   message,
		Files:     changes,
	}); err != nil {
		log.Error("PromoteToLFS ChangeRepoFiles: %v", err)
		ctx.APIErrorInternal(err)
		return
	}

	moved := make([]string, 0, len(changes))
	for _, change := range changes {
		moved = append(moved, change.TreePath)
	}
	ctx.JSON(http.StatusOK, &api.PromoteToLFSResponse{Moved: moved})
}

// deduplicate keeps the first occurrence of each path. ChangeRepoFiles refuses
// a duplicate TreePath, and a caller listing the same file twice means it once.
func deduplicate(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	return unique
}
