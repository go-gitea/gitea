// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	notify_service "code.gitea.io/gitea/services/notify"
	wiki_service "code.gitea.io/gitea/services/wiki"
)

// NewWikiPage response for wiki create request
func NewWikiPage(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/wiki/new repository repoCreateWikiPage
	// ---
	// summary: Create a wiki page
	// consumes:
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
	//   schema:
	//     "$ref": "#/definitions/CreateWikiPageOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/WikiPage"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	if util.IsEmptyString(form.Title) {
		ctx.Error(http.StatusBadRequest, "emptyTitle", nil)
		return
	}

	wikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(form.Message) == 0 {
		form.Message = fmt.Sprintf("Add %q", form.Title)
	}

	content, err := base64.StdEncoding.DecodeString(form.ContentBase64)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "invalid base64 encoding of content", err)
		return
	}
	form.ContentBase64 = string(content)

	if err := wiki_service.AddWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName, form.ContentBase64, form.Message); err != nil {
		if repo_model.IsErrWikiReservedName(err) {
			ctx.Error(http.StatusBadRequest, "IsErrWikiReservedName", err)
		} else if repo_model.IsErrWikiAlreadyExist(err) {
			ctx.Error(http.StatusBadRequest, "IsErrWikiAlreadyExists", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "AddWikiPage", err)
		}
		return
	}

	wikiPage := getWikiPage(ctx, wikiName)

	if !ctx.Written() {
		notify_service.NewWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName), form.Message)
		ctx.JSON(http.StatusCreated, wikiPage)
	}
}

// EditWikiPage response for wiki modify request
func EditWikiPage(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/wiki/page/{pageName} repository repoEditWikiPage
	// ---
	// summary: Edit a wiki page
	// consumes:
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateWikiPageOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPage"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	oldWikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw(":pageName"))
	newWikiName := wiki_service.UserTitleToWebPath("", form.Title)

	if len(newWikiName) == 0 {
		newWikiName = oldWikiName
	}

	if len(form.Message) == 0 {
		form.Message = fmt.Sprintf("Update %q", newWikiName)
	}

	content, err := base64.StdEncoding.DecodeString(form.ContentBase64)
	if err != nil {
		ctx.Error(http.StatusBadRequest, "invalid base64 encoding of content", err)
		return
	}
	form.ContentBase64 = string(content)

	if err := wiki_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, oldWikiName, newWikiName, form.ContentBase64, form.Message); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditWikiPage", err)
		return
	}

	wikiPage := getWikiPage(ctx, newWikiName)

	if !ctx.Written() {
		notify_service.EditWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(newWikiName), form.Message)
		ctx.JSON(http.StatusOK, wikiPage)
	}
}

func getWikiPage(ctx *context.APIContext, wikiName wiki_service.WebPath) *api.WikiPage {
	wikiRepo, commit := findWikiRepoCommit(ctx)
	if wikiRepo != nil {
		defer wikiRepo.Close()
	}
	if ctx.Written() {
		return nil
	}

	// lookup filename in wiki - get filecontent, real filename
	content, pageFilename := wikiContentsByName(ctx, commit, wikiName, false)
	if ctx.Written() {
		return nil
	}

	sidebarContent, _ := wikiContentsByName(ctx, commit, "_Sidebar", true)
	if ctx.Written() {
		return nil
	}

	footerContent, _ := wikiContentsByName(ctx, commit, "_Footer", true)
	if ctx.Written() {
		return nil
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount("master", pageFilename)

	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(pageFilename)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitByPath", err)
		return nil
	}

	return &api.WikiPage{
		WikiPageMetaData: wiki_service.ToWikiPageMetaData(wikiName, lastCommit, ctx.Repo.Repository),
		ContentBase64:    content,
		CommitCount:      commitsCount,
		Sidebar:          sidebarContent,
		Footer:           footerContent,
	}
}

// DeleteWikiPage delete wiki page
func DeleteWikiPage(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/wiki/page/{pageName} repository repoDeleteWikiPage
	// ---
	// summary: Delete a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "423":
	//     "$ref": "#/responses/repoArchivedError"

	wikiName := wiki_service.WebPathFromRequest(ctx.PathParamRaw(":pageName"))

	if err := wiki_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, wikiName); err != nil {
		if err.Error() == "file does not exist" {
			ctx.NotFound(err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "DeleteWikiPage", err)
		return
	}

	notify_service.DeleteWikiPage(ctx, ctx.Doer, ctx.Repo.Repository, string(wikiName))

	ctx.Status(http.StatusNoContent)
}

// ListWikiPages get wiki pages list
func ListWikiPages(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/pages repository repoGetWikiPages
	// ---
	// summary: Get all wiki pages
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPageList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	wikiRepo, commit := findWikiRepoCommit(ctx)
	if wikiRepo != nil {
		defer wikiRepo.Close()
	}
	if ctx.Written() {
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}
	limit := ctx.FormInt("limit")
	if limit <= 1 {
		limit = setting.API.DefaultPagingNum
	}

	skip := (page - 1) * limit
	max := page * limit

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	pages := make([]*api.WikiPageMetaData, 0, len(entries))
	for i, entry := range entries {
		if i < skip || i >= max || !entry.IsRegular() {
			continue
		}
		c, err := wikiRepo.GetCommitByPath(entry.Name())
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommit", err)
			return
		}
		wikiName, err := wiki_service.GitPathToWebPath(entry.Name())
		if err != nil {
			if repo_model.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.Error(http.StatusInternalServerError, "WikiFilenameToName", err)
			return
		}
		pages = append(pages, wiki_service.ToWikiPageMetaData(wikiName, c, ctx.Repo.Repository))
	}

	ctx.SetTotalCountHeader(int64(len(entries)))
	ctx.JSON(http.StatusOK, pages)
}

// GetWikiPage get single wiki page
func GetWikiPage(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/page/{pageName} repository repoGetWikiPage
	// ---
	// summary: Get a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiPage"
	//   "404":
	//     "$ref": "#/responses/notFound"

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw(":pageName"))

	wikiPage := getWikiPage(ctx, pageName)
	if !ctx.Written() {
		ctx.JSON(http.StatusOK, wikiPage)
	}
}

// ListPageRevisions renders file revision list of wiki page
func ListPageRevisions(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/wiki/revisions/{pageName} repository repoGetWikiPageRevisions
	// ---
	// summary: Get revisions of a wiki page
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
	// - name: pageName
	//   in: path
	//   description: name of the page
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/WikiCommitList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	wikiRepo, commit := findWikiRepoCommit(ctx)
	if wikiRepo != nil {
		defer wikiRepo.Close()
	}
	if ctx.Written() {
		return
	}

	// get requested pagename
	pageName := wiki_service.WebPathFromRequest(ctx.PathParamRaw(":pageName"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	// lookup filename in wiki - get filecontent, gitTree entry , real filename
	_, pageFilename := wikiContentsByName(ctx, commit, pageName, false)
	if ctx.Written() {
		return
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount("master", pageFilename)

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	// get Commit Count
	commitsHistory, err := wikiRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: "master",
			File:     pageFilename,
			Page:     page,
		})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CommitsByFileAndRange", err)
		return
	}

	ctx.SetTotalCountHeader(commitsCount)
	ctx.JSON(http.StatusOK, convert.ToWikiCommitList(commitsHistory, commitsCount))
}

// findEntryForFile finds the tree entry for a target filepath.
func findEntryForFile(commit *git.Commit, target string) (*git.TreeEntry, error) {
	entry, err := commit.GetTreeEntryByPath(target)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		return entry, nil
	}

	// Then the unescaped, shortest alternative
	var unescapedTarget string
	if unescapedTarget, err = url.QueryUnescape(target); err != nil {
		return nil, err
	}
	return commit.GetTreeEntryByPath(unescapedTarget)
}

// findWikiRepoCommit opens the wiki repo and returns the latest commit, writing to context on error.
// The caller is responsible for closing the returned repo again
func findWikiRepoCommit(ctx *context.APIContext) (*git.Repository, *git.Commit) {
	wikiRepo, err := gitrepo.OpenWikiRepository(ctx, ctx.Repo.Repository)
	if err != nil {
		if git.IsErrNotExist(err) || err.Error() == "no such file or directory" {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
		}
		return nil, nil
	}

	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
		}
		return wikiRepo, nil
	}
	return wikiRepo, commit
}

// wikiContentsByEntry returns the contents of the wiki page referenced by the
// given tree entry, encoded with base64. Writes to ctx if an error occurs.
func wikiContentsByEntry(ctx *context.APIContext, entry *git.TreeEntry) string {
	blob := entry.Blob()
	if blob.Size() > setting.API.DefaultMaxBlobSize {
		return ""
	}
	content, err := blob.GetBlobContentBase64()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBlobContentBase64", err)
		return ""
	}
	return content
}

// wikiContentsByName returns the contents of a wiki page, along with a boolean
// indicating whether the page exists. Writes to ctx if an error occurs.
func wikiContentsByName(ctx *context.APIContext, commit *git.Commit, wikiName wiki_service.WebPath, isSidebarOrFooter bool) (string, string) {
	gitFilename := wiki_service.WebPathToGitPath(wikiName)
	entry, err := findEntryForFile(commit, gitFilename)
	if err != nil {
		if git.IsErrNotExist(err) {
			if !isSidebarOrFooter {
				ctx.NotFound()
			}
		} else {
			ctx.ServerError("findEntryForFile", err)
		}
		return "", ""
	}
	return wikiContentsByEntry(ctx, entry), gitFilename
}
