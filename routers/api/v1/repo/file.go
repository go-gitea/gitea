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
	db_model "code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_permission "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	repo_service "code.gitea.io/gitea/services/repository"
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

	if err := common.ServeBlob(ctx.Context, blob, lastModified); err != nil {
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
		if err := common.ServeBlob(ctx.Context, blob, lastModified); err != nil {
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

	buf, err := io.ReadAll(dataRc)
	if err != nil {
		_ = dataRc.Close()
		ctx.ServerError("DataAsync", err)
		return
	}

	if err := dataRc.Close(); err != nil {
		log.Error("Error whilst closing blob %s reader in %-v. Error: %v", blob.ID, ctx.Context.Repo.Repository, err)
	}

	// Check if the blob represents a pointer
	pointer, _ := lfs.ReadPointer(bytes.NewReader(buf))

	// if its not a pointer just serve the data directly
	if !pointer.IsValid() {
		// First handle caching for the blob
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		// OK not cached - serve!
		if err := common.ServeData(ctx.Context, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf)); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	}

	// Now check if there is a meta object for this pointer
	meta, err := git_model.GetLFSMetaObjectByOid(ctx.Repo.Repository.ID, pointer.Oid)

	// If there isn't one just serve the data directly
	if err == git_model.ErrLFSObjectNotExist {
		// Handle caching for the blob SHA (not the LFS object OID)
		if httpcache.HandleGenericETagTimeCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`, lastModified) {
			return
		}

		if err := common.ServeData(ctx.Context, ctx.Repo.TreePath, blob.Size(), bytes.NewReader(buf)); err != nil {
			ctx.ServerError("ServeBlob", err)
		}
		return
	} else if err != nil {
		ctx.ServerError("GetLFSMetaObjectByOid", err)
		return
	}

	// Handle caching for the LFS object OID
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+pointer.Oid+`"`) {
		return
	}

	if setting.LFS.ServeDirect {
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

	if err := common.ServeData(ctx.Context, ctx.Repo.TreePath, meta.Size, lfsDataRc); err != nil {
		ctx.ServerError("ServeData", err)
	}
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
		head, err := gitRepo.GetHEADBranch()
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetHEADBranch", err)
			return
		}
		baseCommit, err := gitRepo.GetBranchCommit(head.Name)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
			return
		}

		ctx.Resp.Header().Set("last-commit-id", baseCommit.ID.String())

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
	if setting.RepoArchive.ServeDirect {
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

	ec, err := ctx.Repo.GetEditorconfig(ctx.Repo.Commit)
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

	opts := &files_service.UpdateRepoFileOptions{
		Content:   apiOpts.Content,
		IsNewFile: true,
		Message:   apiOpts.Message,
		TreePath:  ctx.Params("*"),
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
		opts.Message = ctx.Tr("repo.editor.add", opts.TreePath)
	}

	if fileResponse, err := createOrUpdateFile(ctx, opts); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}

// UpdateFile handles API call for updating a file
func UpdateFile(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/contents repository repoUpdateFile
	// ---
	// summary: Update the files in a repository
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
	//     "$ref": "#/definitions/PushFilesOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/error"

	apiOpts := web.GetForm(ctx).(*api.PushFilesOptions)
	repoFilesOptions := &files_service.UpdateRepoFilesOptions{

		Author: &files_service.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Committer: &files_service.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Dates: &files_service.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
		NewBranch: apiOpts.NewBranchName,
		OldBranch: apiOpts.BranchName,
		Signoff:   apiOpts.Signoff,
		Message:   apiOpts.Message,
	}
	if repoFilesOptions.Dates.Author.IsZero() {
		repoFilesOptions.Dates.Author = time.Now()
	}
	if repoFilesOptions.Dates.Committer.IsZero() {
		repoFilesOptions.Dates.Committer = time.Now()
	}
	for i := 0; i < len(apiOpts.Files); i++ {
		fileAction := strings.ToLower(apiOpts.Files[i].FileAction)
		if fileAction == files_service.CreateFileAction || fileAction == files_service.EditFileAction || fileAction == files_service.DeleteFileAction {

			opts := &files_service.UpdateRepoFileOptions{
				FromTreePath: apiOpts.Files[i].FromPath,
				TreePath:     apiOpts.Files[i].FromPath,
				FileAction:   fileAction,
			}

			if fileAction != files_service.DeleteFileAction {
				if len(apiOpts.Files[i].Content) == 0 {
					ctx.Error(http.StatusBadRequest, "ContentIsEmpty", fmt.Errorf("content is empty"))
					return
				}
				opts.Content = apiOpts.Files[i].Content
			}

			if fileAction == files_service.CreateFileAction {
				opts.IsNewFile = true
			} else {
				if len(apiOpts.Files[i].SHA) == 0 {
					ctx.Error(http.StatusBadRequest, "SHAIsEmpty", fmt.Errorf("SHA is empty"))
					return
				}
				opts.SHA = apiOpts.Files[i].SHA
				if fileAction == files_service.EditFileAction {
					opts.IsNewFile = false
					opts.Content = apiOpts.Files[i].Content
				}
			}

			repoFilesOptions.Files = append(repoFilesOptions.Files, opts)

		} else {
			ctx.Error(http.StatusUnprocessableEntity, "FileAction", fmt.Errorf("file action not valid"))
			return
		}
	}
	if ctx.Req.URL.Query().Get("isNewRepo") == "true" {

		owner, err := user_model.GetUserByName(ctx, ctx.Params("username"))
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.JSON(http.StatusNotFound, map[string]interface{}{
					"error": "request owner `" + ctx.Params("username") + "` does not exist",
				})
				return
			}
			ctx.Error(http.StatusUnprocessableEntity, "GetUserByName", err)
			return
		}

		if apiOpts.Readme == "" {
			apiOpts.Readme = "Default"
		}
		if apiOpts.DefaultBranch == "" {
			apiOpts.DefaultBranch = "master"
		}

		repo, err := repo_service.CreateRepository(ctx.Doer, owner, repo_module.CreateRepoOptions{
			Name:          ctx.Params("reponame"),
			Description:   apiOpts.Description,
			IssueLabels:   apiOpts.IssueLabels,
			Gitignores:    apiOpts.Gitignores,
			License:       apiOpts.License,
			Readme:        apiOpts.Readme,
			IsPrivate:     apiOpts.Private,
			AutoInit:      true,
			DefaultBranch: apiOpts.DefaultBranch,
			TrustModel:    repo_model.ToTrustModel(apiOpts.TrustModel),
			IsTemplate:    apiOpts.Template,
		})
		if err != nil {
			if repo_model.IsErrRepoAlreadyExist(err) {
				ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
			} else if db_model.IsErrNameReserved(err) ||
				db_model.IsErrNamePatternNotAllowed(err) ||
				repo_module.IsErrIssueLabelTemplateLoad(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "CreateRepository", err)
			}
			return
		}
		// reload repo from db to get a real state after creation
		repo, err = repo_model.GetRepositoryByID(repo.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetRepositoryByID", err)
		}
		ctx.Repo.Repository = repo
		ctx.Repo.Permission, err = repo_permission.GetUserRepoPermission(ctx, repo, ctx.Doer)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}
	}
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusUnprocessableEntity, "RepoIsEmpty", fmt.Errorf("repo is empty"))
		return
	}
	if apiOpts.BranchName == "" {
		repoFilesOptions.OldBranch = apiOpts.DefaultBranch
	}

	if apiOpts.Message == "" {
		//set message of commit with final changing on commit
		apiOpts.Message = ctx.Tr("repo.editor."+apiOpts.Files[len(apiOpts.Files)-1].FileAction, apiOpts.Files[len(apiOpts.Files)-1].FromPath)
	}

	if fileResponse, err := createOrUpdateOrDeleteFiles(ctx, repoFilesOptions); err != nil {
		handleCreateOrUpdateFileError(ctx, err)
	} else {
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
func createOrUpdateFile(ctx *context.APIContext, opts *files_service.UpdateRepoFileOptions) (*api.FileResponse, error) {
	if !canWriteFiles(ctx, opts.OldBranch) {
		return nil, repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		}
	}

	content, err := base64.StdEncoding.DecodeString(opts.Content)
	if err != nil {
		return nil, err
	}
	opts.Content = string(content)

	return files_service.CreateOrUpdateRepoFile(ctx, ctx.Repo.Repository, ctx.Doer, opts)
}

func createOrUpdateOrDeleteFiles(ctx *context.APIContext, opts *files_service.UpdateRepoFilesOptions) (*files_service.PushedFilesRes, error) {
	if !canWriteFiles(ctx, opts.OldBranch) {
		return nil, repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		}
	}
	for i := 0; i < len(opts.Files); i++ {

		content, err := base64.StdEncoding.DecodeString(opts.Files[i].Content)
		if err != nil {
			return nil, err
		}
		opts.Files[i].Content = string(content)
	}

	return files_service.CreateOrUpdateOrDeleteRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, opts)
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

	opts := &files_service.DeleteRepoFileOptions{
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		SHA:       apiOpts.SHA,
		TreePath:  ctx.Params("*"),
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
		opts.Message = ctx.Tr("repo.editor.delete", opts.TreePath)
	}

	if fileResponse, err := files_service.DeleteRepoFile(ctx, ctx.Repo.Repository, ctx.Doer, opts); err != nil {
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
	// GetContents(ctx)
	if !canReadFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", repo_model.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.Doer.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
		return
	}

	treePath := ctx.Params("*")
	ref := ctx.FormTrim("ref")

	fileList, err := files_service.GetContentsOrList(ctx, ctx.Repo.Repository, treePath, ref)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetContentsOrList", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetContentsOrList", err)
	}

	var allDir []string
	dir := files_service.ContentTypeDir

	for i := 0; i < len(fileList); i++ {
		if fileList[i].Type == dir.String() {
			allDir = append(allDir, fileList[i].Path)
		}
	}

	for i := 0; i < len(allDir); i++ {

		filesDirList, err := files_service.GetContentsOrList(ctx, ctx.Repo.Repository, allDir[i], ref)
		if err != nil {
			if git.IsErrNotExist(err) {
				ctx.NotFound("GetContentsOrList", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetContentsOrList", err)
		}

		for j := range filesDirList {
			if filesDirList[j].Type != dir.String() {
				fileList = append(fileList, filesDirList[j])
			} else {
				allDir = append(allDir, filesDirList[j].Path)
			}
		}

	}

	for i := 0; i < len(fileList); i++ {
		if fileList[i].Type == dir.String() {
			fileList = append(fileList[:i], fileList[i+1:]...)
			i--
		}
	}

	ctx.JSON(http.StatusOK, fileList)
}
