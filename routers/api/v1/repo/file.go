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
	"strings"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	pull_service "code.gitea.io/gitea/services/pull"
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
	// - application/octet-stream
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
	//   description: path of the file to get, it should be "{ref}/{filepath}". If there is no ref could be inferred, it will be treated as the default branch
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: Returns raw file content.
	//     schema:
	//       type: file
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.APIErrorNotFound()
		return
	}

	blob, entry, lastModified := getBlobForEntry(ctx)
	if ctx.Written() {
		return
	}

	ctx.RespHeader().Set(giteaObjectTypeHeader, string(files_service.GetObjectTypeFromTreeEntry(entry)))

	if err := common.ServeBlob(ctx.Base, ctx.Repo.Repository, ctx.Repo.TreePath, blob, lastModified); err != nil {
		ctx.APIErrorInternal(err)
	}
}

// GetRawFileOrLFS get a file by repo's path, redirecting to LFS if necessary.
func GetRawFileOrLFS(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/media/{filepath} repository repoGetRawFileOrLFS
	// ---
	// summary: Get a file or it's LFS object from a repository
	// produces:
	// - application/octet-stream
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
	//   description: path of the file to get, it should be "{ref}/{filepath}". If there is no ref could be inferred, it will be treated as the default branch
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch"
	//   type: string
	//   required: false
	// responses:
	//   200:
	//     description: Returns raw file content.
	//     schema:
	//       type: file
	//   "404":
	//     "$ref": "#/responses/notFound"

	if ctx.Repo.Repository.IsEmpty {
		ctx.APIErrorNotFound()
		return
	}

	blob, entry, lastModified := getBlobForEntry(ctx)
	if ctx.Written() {
		return
	}

	ctx.RespHeader().Set(giteaObjectTypeHeader, string(files_service.GetObjectTypeFromTreeEntry(entry)))

	// LFS Pointer files are at most 1024 bytes - so any blob greater than 1024 bytes cannot be an LFS file
	if blob.Size() > lfs.MetaFileMaxSize {
		// First handle caching for the blob
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		// If not cached - serve!
		if err := common.ServeBlob(ctx.Base, ctx.Repo.Repository, ctx.Repo.TreePath, blob, lastModified); err != nil {
			ctx.APIErrorInternal(err)
		}
		return
	}

	// OK, now the blob is known to have at most 1024 (lfs pointer max size) bytes,
	// we can simply read this in one go (This saves reading it twice)
	dataRc, err := blob.DataAsync()
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	buf, err := io.ReadAll(dataRc)
	if err != nil {
		_ = dataRc.Close()
		ctx.APIErrorInternal(err)
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

		// If not cached - serve!
		common.ServeContentByReader(ctx.Base, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf))
		return
	}

	// Now check if there is a MetaObject for this pointer
	meta, err := git_model.GetLFSMetaObjectByOid(ctx, ctx.Repo.Repository.ID, pointer.Oid)

	// If there isn't one, just serve the data directly
	if errors.Is(err, git_model.ErrLFSObjectNotExist) {
		// Handle caching for the blob SHA (not the LFS object OID)
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		common.ServeContentByReader(ctx.Base, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf))
		return
	} else if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	// Handle caching for the LFS object OID
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
		return
	}

	if setting.LFS.Storage.ServeDirect() {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.LFS.URL(pointer.RelativePath(), blob.Name(), nil)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	lfsDataRc, err := lfs.ReadMetaObject(meta.Pointer)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	defer lfsDataRc.Close()

	common.ServeContentByReadSeeker(ctx.Base, ctx.Repo.TreePath, lastModified, lfsDataRc)
}

func getBlobForEntry(ctx *context.APIContext) (blob *git.Blob, entry *git.TreeEntry, lastModified *time.Time) {
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound()
		} else {
			ctx.APIErrorInternal(err)
		}
		return nil, nil, nil
	}

	if entry.IsDir() || entry.IsSubModule() {
		ctx.APIErrorNotFound("getBlobForEntry", nil)
		return nil, nil, nil
	}

	latestCommit, err := ctx.Repo.GitRepo.GetTreePathLatestCommit(ctx.Repo.Commit.ID.String(), ctx.Repo.TreePath)
	if err != nil {
		ctx.APIErrorInternal(err)
		return nil, nil, nil
	}
	when := &latestCommit.Committer.When

	return entry.Blob(), entry, when
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

	if ctx.Repo.GitRepo == nil {
		var err error
		ctx.Repo.GitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}

	archiveDownload(ctx)
}

func archiveDownload(ctx *context.APIContext) {
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, ctx.PathParam("*"))
	if err != nil {
		if errors.Is(err, archiver_service.ErrUnknownArchiveFormat{}) {
			ctx.APIError(http.StatusBadRequest, err)
		} else if errors.Is(err, archiver_service.RepoRefNotFoundError{}) {
			ctx.APIError(http.StatusNotFound, err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	archiver, err := aReq.Await(ctx)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	download(ctx, aReq.GetArchiveName(), archiver)
}

func download(ctx *context.APIContext, archiveName string, archiver *repo_model.RepoArchiver) {
	downloadName := ctx.Repo.Repository.Name + "-" + archiveName

	// Add nix format link header so tarballs lock correctly:
	// https://github.com/nixos/nix/blob/56763ff918eb308db23080e560ed2ea3e00c80a7/doc/manual/src/protocols/tarball-fetcher.md
	ctx.Resp.Header().Add("Link", fmt.Sprintf(`<%s/archive/%s.%s?rev=%s>; rel="immutable"`,
		ctx.Repo.Repository.APIURL(),
		archiver.CommitID,
		archiver.Type.String(),
		archiver.CommitID,
	))

	rPath := archiver.RelativePath()
	if setting.RepoArchive.Storage.ServeDirect() {
		// If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.RepoArchives.URL(rPath, downloadName, nil)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	// If we have matched and access to release or issue
	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		ctx.APIErrorInternal(err)
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
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch."
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
			ctx.APIErrorNotFound(err)
		} else {
			ctx.APIErrorInternal(err)
		}
		return
	}

	fileName := ctx.PathParam("filename")
	def, err := ec.GetDefinitionForFilename(fileName)
	if def == nil {
		ctx.APIErrorNotFound(err)
		return
	}
	ctx.JSON(http.StatusOK, def)
}

func base64Reader(s string) (io.ReadSeeker, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

func ReqChangeRepoFileOptionsAndCheck(ctx *context.APIContext) {
	commonOpts := web.GetForm(ctx).(api.FileOptionsInterface).GetFileOptions()
	commonOpts.BranchName = util.IfZero(commonOpts.BranchName, ctx.Repo.Repository.DefaultBranch)
	commonOpts.NewBranchName = util.IfZero(commonOpts.NewBranchName, commonOpts.BranchName)
	if !ctx.Repo.CanWriteToBranch(ctx, ctx.Doer, commonOpts.NewBranchName) && !ctx.IsUserSiteAdmin() {
		ctx.APIError(http.StatusForbidden, "user should have a permission to write to the target branch")
		return
	}
	changeFileOpts := &files_service.ChangeRepoFilesOptions{
		Message:   commonOpts.Message,
		OldBranch: commonOpts.BranchName,
		NewBranch: commonOpts.NewBranchName,
		Committer: &files_service.IdentityOptions{
			GitUserName:  commonOpts.Committer.Name,
			GitUserEmail: commonOpts.Committer.Email,
		},
		Author: &files_service.IdentityOptions{
			GitUserName:  commonOpts.Author.Name,
			GitUserEmail: commonOpts.Author.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    commonOpts.Dates.Author,
			Committer: commonOpts.Dates.Committer,
		},
		Signoff: commonOpts.Signoff,
	}
	if commonOpts.Dates.Author.IsZero() {
		commonOpts.Dates.Author = time.Now()
	}
	if commonOpts.Dates.Committer.IsZero() {
		commonOpts.Dates.Committer = time.Now()
	}
	ctx.Data["__APIChangeRepoFilesOptions"] = changeFileOpts
}

func getAPIChangeRepoFileOptions[T api.FileOptionsInterface](ctx *context.APIContext) (apiOpts T, opts *files_service.ChangeRepoFilesOptions) {
	return web.GetForm(ctx).(T), ctx.Data["__APIChangeRepoFilesOptions"].(*files_service.ChangeRepoFilesOptions)
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"
	apiOpts, opts := getAPIChangeRepoFileOptions[*api.ChangeFilesOptions](ctx)
	if ctx.Written() {
		return
	}
	for _, file := range apiOpts.Files {
		contentReader, err := base64Reader(file.ContentBase64)
		if err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, err)
			return
		}
		// FIXME: ChangeFileOperation.SHA is NOT required for update or delete if last commit is provided in the options
		// But the LastCommitID is not provided in the API options, need to fully fix them in API
		changeRepoFile := &files_service.ChangeRepoFile{
			Operation:     file.Operation,
			TreePath:      file.Path,
			FromTreePath:  file.FromPath,
			ContentReader: contentReader,
			SHA:           file.SHA,
		}
		opts.Files = append(opts.Files, changeRepoFile)
	}

	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
		handleChangeRepoFilesError(ctx, err)
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	apiOpts, opts := getAPIChangeRepoFileOptions[*api.CreateFileOptions](ctx)
	if ctx.Written() {
		return
	}
	contentReader, err := base64Reader(apiOpts.ContentBase64)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	}

	opts.Files = append(opts.Files, &files_service.ChangeRepoFile{
		Operation:     "create",
		TreePath:      ctx.PathParam("*"),
		ContentReader: contentReader,
	})
	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
		handleChangeRepoFilesError(ctx, err)
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
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	apiOpts, opts := getAPIChangeRepoFileOptions[*api.UpdateFileOptions](ctx)
	if ctx.Written() {
		return
	}
	contentReader, err := base64Reader(apiOpts.ContentBase64)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	}
	opts.Files = append(opts.Files, &files_service.ChangeRepoFile{
		Operation:     "update",
		ContentReader: contentReader,
		SHA:           apiOpts.SHA,
		FromTreePath:  apiOpts.FromPath,
		TreePath:      ctx.PathParam("*"),
	})
	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
		handleChangeRepoFilesError(ctx, err)
	} else {
		fileResponse := files_service.GetFileResponseFromFilesResponse(filesResponse, 0)
		ctx.JSON(http.StatusOK, fileResponse)
	}
}

func handleChangeRepoFilesError(ctx *context.APIContext, err error) {
	if files_service.IsErrUserCannotCommit(err) || pull_service.IsErrFilePathProtected(err) {
		ctx.APIError(http.StatusForbidden, err)
		return
	}
	if git_model.IsErrBranchAlreadyExists(err) || files_service.IsErrFilenameInvalid(err) || pull_service.IsErrSHADoesNotMatch(err) ||
		files_service.IsErrFilePathInvalid(err) || files_service.IsErrRepoFileAlreadyExists(err) ||
		files_service.IsErrCommitIDDoesNotMatch(err) || files_service.IsErrSHAOrCommitIDNotProvided(err) {
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	}
	if git.IsErrBranchNotExist(err) || files_service.IsErrRepoFileDoesNotExist(err) || git.IsErrNotExist(err) {
		ctx.APIError(http.StatusNotFound, err)
		return
	}
	if errors.Is(err, util.ErrNotExist) {
		ctx.APIError(http.StatusNotFound, err)
		return
	}
	ctx.APIErrorInternal(err)
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
		case "update", "upload", "rename": // upload and rename works like "update", there is no translation for them at the moment
			updateFiles = append(updateFiles, file.TreePath)
		case "delete":
			deleteFiles = append(deleteFiles, file.TreePath)
		}
	}
	message := ""
	if len(createFiles) != 0 {
		message += ctx.Locale.TrString("repo.editor.add", strings.Join(createFiles, ", ")+"\n")
	}
	if len(updateFiles) != 0 {
		message += ctx.Locale.TrString("repo.editor.update", strings.Join(updateFiles, ", ")+"\n")
	}
	if len(deleteFiles) != 0 {
		message += ctx.Locale.TrString("repo.editor.delete", strings.Join(deleteFiles, ", "))
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
	//   "422":
	//     "$ref": "#/responses/error"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	apiOpts, opts := getAPIChangeRepoFileOptions[*api.DeleteFileOptions](ctx)
	if ctx.Written() {
		return
	}

	opts.Files = append(opts.Files, &files_service.ChangeRepoFile{
		Operation: "delete",
		SHA:       apiOpts.SHA,
		TreePath:  ctx.PathParam("*"),
	})
	if opts.Message == "" {
		opts.Message = changeFilesCommitMessage(ctx, opts.Files)
	}

	if filesResponse, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
		handleChangeRepoFilesError(ctx, err)
	} else {
		fileResponse := files_service.GetFileResponseFromFilesResponse(filesResponse, 0)
		ctx.JSON(http.StatusOK, fileResponse) // FIXME on APIv2: return http.StatusNoContent
	}
}

func resolveRefCommit(ctx *context.APIContext, ref string, minCommitIDLen ...int) *utils.RefCommit {
	ref = util.IfZero(ref, ctx.Repo.Repository.DefaultBranch)
	refCommit, err := utils.ResolveRefCommit(ctx, ctx.Repo.Repository, ref, minCommitIDLen...)
	if errors.Is(err, util.ErrNotExist) {
		ctx.APIErrorNotFound(err)
	} else if err != nil {
		ctx.APIErrorInternal(err)
	}
	return refCommit
}

func GetContentsExt(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents-ext/{filepath} repository repoGetContentsExt
	// ---
	// summary: The extended "contents" API, to get file metadata and/or content, or list a directory.
	// description: It guarantees that only one of the response fields is set if the request succeeds.
	//              Users can pass "includes=file_content" or "includes=lfs_metadata" to retrieve more fields.
	//              "includes=file_content" only works for single file, if you need to retrieve file contents in batch,
	//              use "file-contents" API after listing the directory.
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
	//   description: the name of the commit/branch/tag, default to the repository’s default branch.
	//   type: string
	//   required: false
	// - name: includes
	//   in: query
	//   description: By default this API's response only contains file's metadata. Use comma-separated "includes" options to retrieve more fields.
	//                Option "file_content" will try to retrieve the file content, option "lfs_metadata" will try to retrieve LFS metadata.
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsExtResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := files_service.GetContentsOrListOptions{TreePath: ctx.PathParam("*")}
	for includeOpt := range strings.SplitSeq(ctx.FormString("includes"), ",") {
		if includeOpt == "" {
			continue
		}
		switch includeOpt {
		case "file_content":
			opts.IncludeSingleFileContent = true
		case "lfs_metadata":
			opts.IncludeLfsMetadata = true
		default:
			ctx.APIError(http.StatusBadRequest, fmt.Sprintf("unknown include option %q", includeOpt))
			return
		}
	}
	ctx.JSON(http.StatusOK, getRepoContents(ctx, opts))
}

func GetContents(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents/{filepath} repository repoGetContents
	// ---
	// summary: Gets the metadata and contents (if a file) of an entry in a repository, or a list of entries if a dir.
	// description: This API follows GitHub's design, and it is not easy to use. Recommend users to use the "contents-ext" API instead.
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
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch."
	//   type: string
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ret := getRepoContents(ctx, files_service.GetContentsOrListOptions{TreePath: ctx.PathParam("*"), IncludeSingleFileContent: true})
	if ctx.Written() {
		return
	}
	ctx.JSON(http.StatusOK, util.Iif[any](ret.FileContents != nil, ret.FileContents, ret.DirContents))
}

func getRepoContents(ctx *context.APIContext, opts files_service.GetContentsOrListOptions) *api.ContentsExtResponse {
	refCommit := resolveRefCommit(ctx, ctx.FormTrim("ref"))
	if ctx.Written() {
		return nil
	}
	ret, err := files_service.GetContentsOrList(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, refCommit, opts)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.APIErrorNotFound("GetContentsOrList", err)
			return nil
		}
		ctx.APIErrorInternal(err)
	}
	return &ret
}

func GetContentsList(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/contents repository repoGetContentsList
	// ---
	// summary: Gets the metadata of all the entries of the root dir.
	// description: This API follows GitHub's design, and it is not easy to use. Recommend users to use our "contents-ext" API instead.
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
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch."
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

func GetFileContentsGet(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/file-contents repository repoGetFileContents
	// ---
	// summary: Get the metadata and contents of requested files
	// description: See the POST method. This GET method supports using JSON encoded request body in query parameter.
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
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch."
	//   type: string
	//   required: false
	// - name: body
	//   in: query
	//   description: "The JSON encoded body (see the POST request): {\"files\": [\"filename1\", \"filename2\"]}"
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// The POST method requires "write" permission, so we also support this "GET" method
	handleGetFileContents(ctx)
}

func GetFileContentsPost(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/file-contents repository repoGetFileContentsPost
	// ---
	// summary: Get the metadata and contents of requested files
	// description: Uses automatic pagination based on default page size and
	// 							max response size and returns the maximum allowed number of files.
	//							Files which could not be retrieved are null. Files which are too large
	//							are being returned with `encoding == null`, `content == null` and `size > 0`,
	//							they can be requested separately by using the `download_url`.
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
	//   description: "The name of the commit/branch/tag. Default to the repository’s default branch."
	//   type: string
	//   required: false
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/GetFilesOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/ContentsListResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// This is actually a "read" request, but we need to accept a "files" list, then POST method seems easy to use.
	// But the permission system requires that the caller must have "write" permission to use POST method.
	// At the moment, there is no other way to get around the permission check, so there is a "GET" workaround method above.
	handleGetFileContents(ctx)
}

func handleGetFileContents(ctx *context.APIContext) {
	opts, ok := web.GetForm(ctx).(*api.GetFilesOptions)
	if !ok {
		err := json.Unmarshal(util.UnsafeStringToBytes(ctx.FormString("body")), &opts)
		if err != nil {
			ctx.APIError(http.StatusBadRequest, "invalid body parameter")
			return
		}
	}
	refCommit := resolveRefCommit(ctx, ctx.FormTrim("ref"))
	if ctx.Written() {
		return
	}
	filesResponse := files_service.GetContentsListFromTreePaths(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, refCommit, opts.Files)
	ctx.JSON(http.StatusOK, util.SliceNilAsEmpty(filesResponse))
}
