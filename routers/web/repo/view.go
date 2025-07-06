// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	gocontext "context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	_ "image/gif"  // for processing gif images
	_ "image/jpeg" // for processing jpeg images
	_ "image/png"  // for processing png images

	activities_model "code.gitea.io/gitea/models/activities"
	admin_model "code.gitea.io/gitea/models/admin"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	repo_service "code.gitea.io/gitea/services/repository"

	_ "golang.org/x/image/bmp"  // for processing bmp images
	_ "golang.org/x/image/webp" // for processing webp images
)

const (
	tplRepoEMPTY       templates.TplName = "repo/empty"
	tplRepoHome        templates.TplName = "repo/home"
	tplRepoView        templates.TplName = "repo/view"
	tplRepoViewContent templates.TplName = "repo/view_content"
	tplRepoViewList    templates.TplName = "repo/view_list"
	tplWatchers        templates.TplName = "repo/watchers"
	tplForks           templates.TplName = "repo/forks"
	tplMigrating       templates.TplName = "repo/migrate/migrating"
)

type fileInfo struct {
	fileSize int64
	lfsMeta  *lfs.Pointer
	st       typesniffer.SniffedType
}

func (fi *fileInfo) isLFSFile() bool {
	return fi.lfsMeta != nil && fi.lfsMeta.Oid != ""
}

func getFileReader(ctx gocontext.Context, repoID int64, blob *git.Blob) (buf []byte, dataRc io.ReadCloser, fi *fileInfo, err error) {
	dataRc, err = blob.DataAsync()
	if err != nil {
		return nil, nil, nil, err
	}

	const prefetchSize = lfs.MetaFileMaxSize

	buf = make([]byte, prefetchSize)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]

	fi = &fileInfo{fileSize: blob.Size(), st: typesniffer.DetectContentType(buf)}

	// FIXME: what happens when README file is an image?
	if !fi.st.IsText() || !setting.LFS.StartServer {
		return buf, dataRc, fi, nil
	}

	pointer, _ := lfs.ReadPointerFromBuffer(buf)
	if !pointer.IsValid() { // fallback to a plain file
		return buf, dataRc, fi, nil
	}

	meta, err := git_model.GetLFSMetaObjectByOid(ctx, repoID, pointer.Oid)
	if err != nil { // fallback to a plain file
		log.Warn("Unable to access LFS pointer %s in repo %d: %v", pointer.Oid, repoID, err)
		return buf, dataRc, fi, nil
	}

	// close the old dataRc and open the real LFS target
	_ = dataRc.Close()
	dataRc, err = lfs.ReadMetaObject(pointer)
	if err != nil {
		return nil, nil, nil, err
	}

	buf = make([]byte, prefetchSize)
	n, err = util.ReadAtMost(dataRc, buf)
	if err != nil {
		_ = dataRc.Close()
		return nil, nil, fi, err
	}
	buf = buf[:n]
	fi.st = typesniffer.DetectContentType(buf)
	fi.fileSize = blob.Size()
	fi.lfsMeta = &meta.Pointer
	return buf, dataRc, fi, nil
}

func loadLatestCommitData(ctx *context.Context, latestCommit *git.Commit) bool {
	// Show latest commit info of repository in table header,
	// or of directory if not in root directory.
	ctx.Data["LatestCommit"] = latestCommit
	if latestCommit != nil {
		verification := asymkey_service.ParseCommitWithSignature(ctx, latestCommit)

		if err := asymkey_model.CalculateTrustStatus(verification, ctx.Repo.Repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
			return repo_model.IsOwnerMemberCollaborator(ctx, ctx.Repo.Repository, user.ID)
		}, nil); err != nil {
			ctx.ServerError("CalculateTrustStatus", err)
			return false
		}
		ctx.Data["LatestCommitVerification"] = verification
		ctx.Data["LatestCommitUser"] = user_model.ValidateCommitWithEmail(ctx, latestCommit)

		statuses, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, latestCommit.ID.String(), db.ListOptionsAll)
		if err != nil {
			log.Error("GetLatestCommitStatus: %v", err)
		}
		if !ctx.Repo.CanRead(unit_model.TypeActions) {
			git_model.CommitStatusesHideActionsURL(ctx, statuses)
		}

		ctx.Data["LatestCommitStatus"] = git_model.CalcCommitStatus(statuses)
		ctx.Data["LatestCommitStatuses"] = statuses
	}

	return true
}

func markupRender(ctx *context.Context, renderCtx *markup.RenderContext, input io.Reader) (escaped *charset.EscapeStatus, output template.HTML, err error) {
	markupRd, markupWr := io.Pipe()
	defer markupWr.Close()
	done := make(chan struct{})
	go func() {
		sb := &strings.Builder{}
		// We allow NBSP here this is rendered
		escaped, _ = charset.EscapeControlReader(markupRd, sb, ctx.Locale, charset.RuneNBSP)
		output = template.HTML(sb.String())
		close(done)
	}()
	err = markup.Render(renderCtx, input, markupWr)
	_ = markupWr.CloseWithError(err)
	<-done
	return escaped, output, err
}

func checkHomeCodeViewable(ctx *context.Context) {
	if ctx.Repo.HasUnits() {
		if ctx.Repo.Repository.IsBeingCreated() {
			task, err := admin_model.GetMigratingTask(ctx, ctx.Repo.Repository.ID)
			if err != nil {
				if admin_model.IsErrTaskDoesNotExist(err) {
					ctx.Data["Repo"] = ctx.Repo
					ctx.Data["CloneAddr"] = ""
					ctx.Data["Failed"] = true
					ctx.HTML(http.StatusOK, tplMigrating)
					return
				}
				ctx.ServerError("models.GetMigratingTask", err)
				return
			}
			cfg, err := task.MigrateConfig()
			if err != nil {
				ctx.ServerError("task.MigrateConfig", err)
				return
			}

			ctx.Data["Repo"] = ctx.Repo
			ctx.Data["MigrateTask"] = task
			ctx.Data["CloneAddr"], _ = util.SanitizeURL(cfg.CloneAddr)
			ctx.Data["Failed"] = task.Status == structs.TaskStatusFailed
			ctx.HTML(http.StatusOK, tplMigrating)
			return
		}

		if ctx.IsSigned {
			// Set repo notification-status read if unread
			if err := activities_model.SetRepoReadBy(ctx, ctx.Repo.Repository.ID, ctx.Doer.ID); err != nil {
				ctx.ServerError("ReadBy", err)
				return
			}
		}

		var firstUnit *unit_model.Unit
		for _, repoUnitType := range ctx.Repo.Permission.ReadableUnitTypes() {
			if repoUnitType == unit_model.TypeCode {
				// we are doing this check in "code" unit related pages, so if the code unit is readable, no need to do any further redirection
				return
			}

			unit, ok := unit_model.Units[repoUnitType]
			if ok && (firstUnit == nil || !firstUnit.IsLessThan(unit)) {
				firstUnit = &unit
			}
		}

		if firstUnit != nil {
			ctx.Redirect(fmt.Sprintf("%s%s", ctx.Repo.Repository.Link(), firstUnit.URI))
			return
		}
	}

	ctx.NotFound(errors.New(ctx.Locale.TrString("units.error.no_unit_allowed_repo")))
}

// LastCommit returns lastCommit data for the provided branch/tag/commit and directory (in url) and filenames in body
func LastCommit(ctx *context.Context) {
	checkHomeCodeViewable(ctx)
	if ctx.Written() {
		return
	}

	renderDirectoryFiles(ctx, 0)
	if ctx.Written() {
		return
	}

	var treeNames []string
	paths := make([]string, 0, 5)
	if len(ctx.Repo.TreePath) > 0 {
		treeNames = strings.Split(ctx.Repo.TreePath, "/")
		for i := range treeNames {
			paths = append(paths, strings.Join(treeNames[:i+1], "/"))
		}

		ctx.Data["HasParentPath"] = true
		if len(paths)-2 >= 0 {
			ctx.Data["ParentPath"] = "/" + paths[len(paths)-2]
		}
	}
	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()
	ctx.Data["BranchLink"] = branchLink

	ctx.HTML(http.StatusOK, tplRepoViewList)
}

func prepareDirectoryFileIcons(ctx *context.Context, files []git.CommitInfo) {
	renderedIconPool := fileicon.NewRenderedIconPool()
	fileIcons := map[string]template.HTML{}
	for _, f := range files {
		fullPath := path.Join(ctx.Repo.TreePath, f.Entry.Name())
		entryInfo := fileicon.EntryInfoFromGitTreeEntry(ctx.Repo.Commit, fullPath, f.Entry)
		fileIcons[f.Entry.Name()] = fileicon.RenderEntryIconHTML(renderedIconPool, entryInfo)
	}
	fileIcons[".."] = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolder())
	ctx.Data["FileIcons"] = fileIcons
	ctx.Data["FileIconPoolHTML"] = renderedIconPool.RenderToHTML()
}

func renderDirectoryFiles(ctx *context.Context, timeout time.Duration) git.Entries {
	tree, err := ctx.Repo.Commit.SubTree(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "Repo.Commit.SubTree", err)
		return nil
	}

	ctx.Data["LastCommitLoaderURL"] = ctx.Repo.RepoLink + "/lastcommit/" + url.PathEscape(ctx.Repo.CommitID) + "/" + util.PathEscapeSegments(ctx.Repo.TreePath)

	// Get current entry user currently looking at.
	entry, err := ctx.Repo.Commit.GetTreeEntryByPath(ctx.Repo.TreePath)
	if err != nil {
		HandleGitError(ctx, "Repo.Commit.GetTreeEntryByPath", err)
		return nil
	}

	if !entry.IsDir() {
		HandleGitError(ctx, "Repo.Commit.GetTreeEntryByPath", err)
		return nil
	}

	allEntries, err := tree.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return nil
	}
	allEntries.CustomSort(base.NaturalSortLess)

	commitInfoCtx := gocontext.Context(ctx)
	if timeout > 0 {
		var cancel gocontext.CancelFunc
		commitInfoCtx, cancel = gocontext.WithTimeout(ctx, timeout)
		defer cancel()
	}

	files, latestCommit, err := allEntries.GetCommitsInfo(commitInfoCtx, ctx.Repo.Commit, ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("GetCommitsInfo", err)
		return nil
	}
	ctx.Data["Files"] = files
	prepareDirectoryFileIcons(ctx, files)
	for _, f := range files {
		if f.Commit == nil {
			ctx.Data["HasFilesWithoutLatestCommit"] = true
			break
		}
	}

	if !loadLatestCommitData(ctx, latestCommit) {
		return nil
	}

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.RefTypeNameSubURL()
	treeLink := branchLink

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + util.PathEscapeSegments(ctx.Repo.TreePath)
	}

	ctx.Data["TreeLink"] = treeLink

	return allEntries
}

// RenderUserCards render a page show users according the input template
func RenderUserCards(ctx *context.Context, total int, getter func(opts db.ListOptions) ([]*user_model.User, error), tpl templates.TplName) {
	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}
	pager := context.NewPagination(total, setting.ItemsPerPage, page, 5)
	ctx.Data["Page"] = pager

	items, err := getter(db.ListOptions{
		Page:     pager.Paginater.Current(),
		PageSize: setting.ItemsPerPage,
	})
	if err != nil {
		ctx.ServerError("getter", err)
		return
	}
	ctx.Data["Cards"] = items

	ctx.HTML(http.StatusOK, tpl)
}

// Watchers render repository's watch users
func Watchers(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.watchers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.watchers")
	RenderUserCards(ctx, ctx.Repo.Repository.NumWatches, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetRepoWatchers(ctx, ctx.Repo.Repository.ID, opts)
	}, tplWatchers)
}

// Stars render repository's starred users
func Stars(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.stargazers")
	ctx.Data["CardsTitle"] = ctx.Tr("repo.stargazers")
	RenderUserCards(ctx, ctx.Repo.Repository.NumStars, func(opts db.ListOptions) ([]*user_model.User, error) {
		return repo_model.GetStargazers(ctx, ctx.Repo.Repository, opts)
	}, tplWatchers)
}

// Forks render repository's forked users
func Forks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.forks")

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}
	pageSize := setting.ItemsPerPage

	forks, total, err := repo_service.FindForks(ctx, ctx.Repo.Repository, ctx.Doer, db.ListOptions{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ctx.ServerError("FindForks", err)
		return
	}

	if err := repo_model.RepositoryList(forks).LoadOwners(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	pager := context.NewPagination(int(total), pageSize, page, 5)
	ctx.Data["ShowRepoOwnerAvatar"] = true
	ctx.Data["ShowRepoOwnerOnList"] = true
	ctx.Data["Page"] = pager
	ctx.Data["Repos"] = forks

	ctx.HTML(http.StatusOK, tplForks)
}
