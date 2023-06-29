// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	archiver_service "code.gitea.io/gitea/services/repository/archiver"
	files_service "code.gitea.io/gitea/services/repository/files"
)

const giteaObjectTypeHeader = "X-Gitea-Object-Type"

// GetRawFile get a file by path on a repository
func GetRawFile(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/raw/{filepath} repository repoGetRawFile
	// ---
	// summary: Get a file from a repository
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
	// - name: filepath
	//   in: path
	//   description: filepath of the file to get
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: Returns raw file content.
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound()
		return
	}

	blob, entry, lastModified := getBlobForEntry(ctx)
	if ctx.Written() {
		return
	}

	ctx.RespHeader().Set(giteaObjectTypeHeader, string(files_service.GetObjectTypeFromTreeEntry(entry)))

	if err := common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, lastModified); err != nil {
		ctx.Error(http.StatusInternalServerError, "ServeBlob", err)
	}
}

// GetRawFileOrLFS get a file by repo's path, redirecting to LFS if necessary.
func GetRawFileOrLFS(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/media/{filepath} repository repoGetRawFileOrLFS
	// ---
	// summary: Get a file or it's LFS object from a repository
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
	// - name: filepath
	//   in: path
	//   description: filepath of the file to get
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: Returns raw file content.
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound()
		return
	}

	blob, entry, lastModified := getBlobForEntry(ctx)
	if ctx.Written() {
		return
	}

	ctx.RespHeader().Set(giteaObjectTypeHeader, string(files_service.GetObjectTypeFromTreeEntry(entry)))

	// LFS Pointer files are at most 1024 bytes - so any blob greater than 1024 bytes cannot be an LFS file
	if blob.Size() > 1024 {
		// First handle caching for the blob
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		// OK not cached - serve!
		if err := common.ServeBlob(ctx.Base, ctx.Repo.TreePath, blob, lastModified); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	}

	// OK, now the blob is known to have at most 1024 bytes we can simply read this in in one go (This saves reading it twice)
	dataRc, err := blob.DataAsync()
	if err != nil {
		ctx.ServerError("DataAsync", err)
		return
	}

	// FIXME: code from #19689, what if the file is large ... OOM ...
	buf, err := io.ReadAll(dataRc)
	if err != nil {
		_ = dataRc.Close()
		ctx.ServerError("DataAsync", err)
		return
	}

	if err := dataRc.Close(); err != nil {
		log.Error("Error whilst closing blob %s reader in %-v. Error: %v", blob.ID, ctx.Repo.Repository, err)
	}

	// Check if the blob represents a pointer
	pointer, _ := lfs.ReadPointer(bytes.NewReader(buf))

	// if it's not a pointer, just serve the data directly
	if !pointer.IsValid() {
		// First handle caching for the blob
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		// OK not cached - serve!
		common.ServeContentByReader(ctx.Base, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf))
		return
	}

	// Now check if there is a MetaObject for this pointer
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, pointer.Oid)

	// If there isn't one, just serve the data directly
	if err == git_model.ErrLFSObjectNotExist {
		// Handle caching for the blob SHA (not the LFS object OID)
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		common.ServeContentByReader(ctx.Base, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf))
		return
	} else if err != nil {
		ctx.ServerError("GetLFSMetaObjectByOid", err)
		return
	}

	// Handle caching for the LFS object OID
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
		return
	}

	if setting.LFS.Storage.MinioConfig.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.LFS.URL(pointer.RelativePath(), blob.Name())
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	lfsDataRc, err := lfs.ReadMetaObject(meta.Pointer)
	if err != nil {
		ctx.ServerError("ReadMetaObject", err)
		return
	}
	defer lfsDataRc.Close()

	common.ServeContentByReadSeeker(ctx.Base, ctx.Repo.TreePath, lastModified, lfsDataRc)
}

func getBlobForEntry(ctx *context.APIContext) (blob *git.Blob, entry *git.TreeEntry, lastModified time.Time) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetTreeEntryByPath", err)
		}
		return
	}

	if entry.IsDir() || entry.IsSubModule() {
		ctx.NotFound("getBlobForEntry", nil)
		return
	}

	info, _, err := git.Entries([]*git.TreeEntry{entry}).GetCommitsInfo(ctx, ctx.Repo.Commit, path.Dir("/" + ctx.Repo.TreePath)[1:])
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitsInfo", err)
		return
	}

	if len(info) == 1 {
		// Not Modified
		lastModified = info[0].Commit.Committer.When
	}
	blob = entry.Blob()

	return blob, entry, lastModified
}

// GetArchive get archive of a repository
func GetArchive(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/archive/{archive} repository repoGetArchive
	// ---
	// summary: Get an archive of a repository
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
	// - name: archive
	//   in: path
	//   description: the git reference for download with attached archive format (e.g. master.zip)
	//   type: string
	//   required: true
	// responses:
	//   200:
	//     description: success
	//   "404":
	//     "$ref": "#/responses/notFound"

	repoPath := repo_model.RepoPath(ctx.Params(":username"), ctx.Params(":reponame"))
	if ctx.Repo.GitRepo == nil {
		gitRepo, err := git.OpenRepository(ctx, repoPath)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
			return
		}
		ctx.Repo.GitRepo = gitRepo
		defer gitRepo.Close()
	}

	archiveDownload(ctx)
}

func archiveDownload(ctx *context.APIContext) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		if errors.Is(err, archiver_service.ErrUnknownArchiveFormat{}) {
			ctx.Error(http.StatusBadRequest, "unknown archive format", err)
		} else if errors.Is(err, archiver_service.RepoRefNotFoundError{}) {
			ctx.Error(http.StatusNotFound, "unrecognized reference", err)
		} else {
			ctx.ServerError("archiver_service.NewRequest", err)
		}
		return
	}

	archiver, err := aReq.Await(ctx)
	if err != nil {
		ctx.ServerError("archiver.Await", err)
		return
	}

	download(ctx, aReq.GetArchiveName(), archiver)
}

func download(ctx *context.APIContext, archiveName string, archiver *repo_model.RepoArchiver) {
	downloadName := ctx.Repo.Repository.Name + "-" + archiveName

	rPath := archiver.RelativePath()
	if setting.RepoArchive.Storage.MinioConfig.ServeDirect {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.RepoArchives.URL(rPath, downloadName)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	// If we have matched and access to release or issue
	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	ctx.ServeContent(fr, &context.ServeHeaderOptions{
		Filename:     downloadName,
		LastModified: archiver.CreatedUnix.AsLocalTime(),
	})
}

// GetEditorconfig get editor config of a repository
func GetEditorconfig(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/editorconfig/{filepath} repository repoGetEditorConfig
	// ---
	// summary: Get the EditorConfig definitions of a file in a repository
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
	// - name: filepath
	//   in: path
	//   description: filepath of file to get
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: success
	//   "404":
	//     "$ref": "#/responses/notFound"

	ec, _, err := ctx.Repo.GetEditorconfig(ctx.Repo.Commit)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetEditorconfig", err)
		}
		return
	}

	fileName := ctx.Params("filename")
	def, err := ec.GetDefinitionForFilename(fileName)
	if def == nil {
		ctx.NotFound(err)
		return
	}
	ctx.JSON(http.StatusOK, def)
}

// canWriteFiles returns true if repository is editable and user has proper access level.
func canWriteFiles(ctx *context.APIContext, branch string) bool {
	return ctx.Repo.CanWriteToBranch(ctx.Doer, branch) &&
		!ctx.Repo.Repository.IsMirror &&
		!ctx.Repo.Repository.IsArchived
}

// canReadFiles returns true if repository is readable and user has proper access level.
func canReadFiles(r *context.Repository) bool {
	return r.Permission.CanRead(unit.TypeCode)
}

// ChangeFiles handles API call for modifying multiple files
func ChangeFiles(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/contents repository repoChangeFiles
	// ---
	// summary: Modify multiple files in a repository
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
	//     "$ref": "#/definitions/ChangeFilesOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FilesResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.ChangeFilesOptions)

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	files := []*files_service.ChangeRepoFile{}
	for _, file := range apiOpts.Files {
		changeRepoFile := &files_service.ChangeRepoFile{
			Operation:    file.Operation,
			TreePath:     file.Path,
			FromTreePath: file.FromPath,
			Content:      file.Content,
			SHA:          file.SHA,
		}
		files = append(files, changeRepoFile)
	}

	opts := &files_service.ChangeRepoFilesOptions{
		Files:     files,
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &files_service.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &files_service.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, files)
	}

	if filesResponse, err := createOrUpdateFiles(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		ctx.JSON(http.StatusCreated, filesResponse)
	}
}

// CreateFile handles API call for creating a file
func CreateFile(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/contents/{filepath} repository repoCreateFile
	// ---
	// summary: Create a file in a repository
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
	// - name: filepath
	//   in: path
	//   description: path of the file to create
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreateFileOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/FileResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.CreateFileOptions)

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "create",
				TreePath:  ctx.Params("*"),
				Content:   apiOpts.Content,
			},
		},
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &files_service.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &files_service.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := createOrUpdateFiles(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		fileResponse := files_service.GetFileResponseFromFilesResponse(filesResponse, 0)
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}

// UpdateFile handles API call for updating a file
func UpdateFile(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/contents/{filepath} repository repoUpdateFile
	// ---
	// summary: Update a file in a repository
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
	// - name: filepath
	//   in: path
	//   description: path of the file to update
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"
	apiOpts := web.GetForm(ctx).(*api.UpdateFileOptions)
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusUnprocessableEntity, "RepoIsEmpty", fmt.Errorf("repo is empty"))
	}

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:    "update",
				Content:      apiOpts.Content,
				SHA:          apiOpts.SHA,
				FromTreePath: apiOpts.FromPath,
				TreePath:     ctx.Params("*"),
			},
		},
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &files_service.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &files_service.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := createOrUpdateFiles(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		fileResponse := files_service.GetFileResponseFromFilesResponse(filesResponse, 0)
		ctx.JSON(http.StatusOK, fileResponse)
	}
}

func handleCreateOrUpdateFileError(ctx *context.APIContext, err error) {
	if models.IsErrUserCannotCommit(err) || models.IsErrFilePathProtected(err) {
		ctx.Error(http.StatusForbidden, "Access", err)
		return
	}
	if models.IsErrBranchAlreadyExists(err) || models.IsErrFilenameInvalid(err) || models.IsErrSHADoesNotMatch(err) ||
		models.IsErrFilePathInvalid(err) || models.IsErrRepoFileAlreadyExists(err) {
		ctx.Error(http.StatusUnprocessableEntity, "Invalid", err)
		return
	}
	if models.IsErrBranchDoesNotExist(err) || git.IsErrBranchNotExist(err) {
		ctx.Error(http.StatusNotFound, "BranchDoesNotExist", err)
		return
	}

	ctx.Error(http.StatusInternalServerError, "UpdateFile", err)
}

// Called from both CreateFile or UpdateFile to handle both
func createOrUpdateFiles(ctx *context.APIContext, opts *files_service.ChangeRepoFilesOptions) (*api.FilesResponse, error) {
	if !canWriteFiles(ctx, opts.OldBranch) {
		return nil, repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		}
	}

	for _, file := range opts.Files {
		content, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return nil, err
		}
		file.Content = string(content)
	}

	return files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts)
}

// format commit message if empty
func changeFilesCommitMessage(ctx *context.APIContext, files []*files_service.ChangeRepoFile) string {
	var (
		createFiles []string
		updateFiles []string
		deleteFiles []string
	)
	for _, file := range files {
		switch file.Operation {
		case "create":
			createFiles = append(createFiles, file.TreePath)
		case "update":
			updateFiles = append(updateFiles, file.TreePath)
		case "delete":
			deleteFiles = append(deleteFiles, file.TreePath)
		}
	}
	message := ""
	if len(createFiles) != 0 {
		message += ctx.Tr("repo.editor.add", strings.Join(createFiles, ", ")+"\n")
	}
	if len(updateFiles) != 0 {
		message += ctx.Tr("repo.editor.update", strings.Join(updateFiles, ", ")+"\n")
	}
	if len(deleteFiles) != 0 {
		message += ctx.Tr("repo.editor.delete", strings.Join(deleteFiles, ", "))
	}
	return strings.Trim(message, "\n")
}

// DeleteFile Delete a file in a repository
func DeleteFile(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/contents/{filepath} repository repoDeleteFile
	// ---
	// summary: Delete a file in a repository
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
	// - name: filepath
	//   in: path
	//   description: path of the file to delete
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/DeleteFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileDeleteResponse"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.DeleteFileOptions)
	if !canWriteFiles(ctx, apiOpts.BranchName) {
		ctx.Error(http.StatusForbidden, "DeleteFile", repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	if apiOpts.BranchName == "" {
		apiOpts.BranchName = ctx.Repo.Repository.DefaultBranch
	}

	opts := &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{
				Operation: "delete",
				SHA:       apiOpts.SHA,
				TreePath:  ctx.Params("*"),
			},
		},
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &files_service.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &files_service.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		Signoff: apiOpts.Signoff,
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
		if git.IsErrBranchNotExist(err) || models.IsErrRepoFileDoesNotExist(err) || git.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "DeleteFile", err)
			return
		} else if models.IsErrBranchAlreadyExists(err) ||
			models.IsErrFilenameInvalid(err) ||
			models.IsErrSHADoesNotMatch(err) ||
			models.IsErrCommitIDDoesNotMatch(err) ||
			models.IsErrSHAOrCommitIDNotProvided(err) {
			ctx.Error(http.StatusBadRequest, "DeleteFile", err)
			return
		} else if models.IsErrUserCannotCommit(err) {
			ctx.Error(http.StatusForbidden, "DeleteFile", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteFile", err)
	} else {
		fileResponse := files_service.GetFileResponseFromFilesResponse(filesResponse, 0)
		ctx.JSON(http.StatusOK, fileResponse) // FIXME on APIv2: return http.StatusNoContent
	}
}

// GetContents Get the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir
func GetContents(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents/{filepath} repository repoGetContents
	// ---
	// summary: Gets the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir
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
	// - name: filepath
	//   in: path
	//   description: path of the dir, file, symlink or submodule in the repo
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !canReadFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	treePath := ctx.Params("*")
	ref := ctx.FormTrim("ref")

	if fileList, err := files_service.GetContentsOrList(ctx, ctx.Repo.Repository, treePath, ref); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetContentsOrList", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", err)
	} else {
		ctx.JSON(http.StatusOK, fileList)
	}
}

// GetContentsList Get the metadata of all the entries of the root dir
func GetContentsList(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents repository repoGetContentsList
	// ---
	// summary: Gets the metadata of all the entries of the root dir
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
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default the repository’s default branch (usually master)"
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// same as GetContents(), this function is here because swagger fails if path is empty in GetContents() interface
	GetContents(ctx)
}
