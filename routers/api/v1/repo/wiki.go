// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"io"
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	if util.IsEmptyString(form.Title) {
		ctx.Error(http.StatusBadRequest, "emptyTitle", nil)
		return
	}

	wikiName := wiki_service.NormalizeWikiName(form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Tr("repo.editor.add", form.Title)
	}

	if err := wiki_service.AddWikiPage(ctx.User, ctx.Repo.Repository, wikiName, form.Content, form.Message); err != nil {
		if models.IsErrWikiReservedName(err) {
			ctx.Error(http.StatusBadRequest, "IsErrWikiReservedName", err)
		} else if models.IsErrWikiAlreadyExist(err) {
			ctx.Error(http.StatusBadRequest, "IsErrWikiAlreadyExists", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "AddWikiPage", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
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
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.CreateWikiPageOptions)

	oldWikiName := wiki_service.NormalizeWikiName(ctx.Params(":pageName"))
	newWikiName := wiki_service.NormalizeWikiName(form.Title)

	if len(form.Message) == 0 {
		form.Message = ctx.Tr("repo.editor.update", form.Title)
	}

	if len(newWikiName) == 0 {
		newWikiName = oldWikiName
	}

	if err := wiki_service.EditWikiPage(ctx.User, ctx.Repo.Repository, oldWikiName, newWikiName, form.Content, form.Message); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditWikiPage", err)
		return
	}

	ctx.Status(http.StatusNoContent)
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

	wikiName := wiki_service.NormalizeWikiName(ctx.Params(":pageName"))
	if len(wikiName) == 0 {
		ctx.Status(http.StatusBadRequest)
		return
	}

	if err := wiki_service.DeleteWikiPage(ctx.User, ctx.Repo.Repository, wikiName); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteWikiPage", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// WikiPages get wiki pages list
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
	//   "201":
	//     "$ref": "#/responses/WikiPageList"

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		return
	}
	if wikiRepo != nil {
		defer wikiRepo.Close()
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	limit := ctx.FormInt("limit")
	if limit <= 1 {
		limit = 20
	}

	skip := (page - 1) * limit
	max := page * limit

	entries, err := commit.ListEntries()
	if err != nil {
		ctx.ServerError("ListEntries", err)
		return
	}
	pages := make([]api.WikiPageMetaData, 0, len(entries))
	for i, entry := range entries {
		if i < skip || i >= max || !entry.IsRegular() {
			continue
		}
		c, err := wikiRepo.GetCommitByPath(entry.Name())
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommit", err)
			return
		}
		wikiName, err := wiki_service.FilenameToName(entry.Name())
		if err != nil {
			if models.IsErrWikiInvalidFileName(err) {
				continue
			}
			ctx.Error(http.StatusInternalServerError, "WikiFilenameToName", err)
			return
		}
		pages = append(pages, api.WikiPageMetaData{
			Name:    wikiName,
			SubURL:  wiki_service.NameToSubURL(wikiName),
			Updated: c.Author.When.Format(time.RFC3339),
		})
	}

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
	//   "201":
	//     "$ref": "#/responses/WikiPage"

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		if git.IsErrNotExist(err) {
			ctx.NotFound(http.StatusInternalServerError, "GetBranchCommit", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
		}
		return
	}

	// get requested pagename
	pageName := wiki_service.NormalizeWikiName(ctx.Params(":pageName"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	//lookup filename in wiki - get filecontent, gitTree entry , real filename
	data, entry, pageFilename, noEntry := wikiContentsByName(ctx, commit, pageName)
	if noEntry {
		ctx.NotFound()
		return
	}
	if entry == nil || ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.NotFound()
		return
	}

	sidebarContent, _, _, _ := wikiContentsByName(ctx, commit, "_Sidebar")
	if ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.NotFound()
		return
	}

	footerContent, _, _, _ := wikiContentsByName(ctx, commit, "_Footer")
	if ctx.Written() {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.NotFound()
		return
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount("master", pageFilename)

	wikiPath := entry.Name()
	// Get last change information.
	lastCommit, err := wikiRepo.GetCommitByPath(wikiPath)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitByPath", err)
		return
	}

	wikiPage := &api.WikiPage{
		WikiPageMetaData: &api.WikiPageMetaData{
			Name:    pageName,
			SubURL:  wiki_service.NameToSubURL(pageName),
			Updated: lastCommit.Author.When.Format(time.RFC3339),
		},
		Content:     string(data),
		CommitCount: commitsCount,
		LastCommit:  convert.ToWikiCommit(lastCommit),
		Sidebar:     string(sidebarContent),
		Footer:      string(footerContent),
	}

	ctx.JSON(http.StatusOK, wikiPage)
}

// WikiRevision renders file revision list of wiki page
func WikiRevision(ctx *context.APIContext) {
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
	//   "201":
	//     "$ref": "#/responses/WikiCommitList"

	wikiRepo, commit, err := findWikiRepoCommit(ctx)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		if git.IsErrNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranchCommit", err)
		}
		return
	}

	// get requested pagename
	pageName := wiki_service.NormalizeWikiName(ctx.Params(":pageName"))
	if len(pageName) == 0 {
		pageName = "Home"
	}

	//lookup filename in wiki - get filecontent, gitTree entry , real filename
	_, _, pageFilename, noEntry := wikiContentsByName(ctx, commit, pageName)
	if noEntry {
		ctx.NotFound()
	}

	// get commit count - wiki revisions
	commitsCount, _ := wikiRepo.FileCommitsCount("master", pageFilename)

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	// get Commit Count
	commitsHistory, err := wikiRepo.CommitsByFileAndRangeNoFollow("master", pageFilename, page)
	if err != nil {
		if wikiRepo != nil {
			wikiRepo.Close()
		}
		ctx.Error(http.StatusInternalServerError, "CommitsByFileAndRangeNoFollow", err)
	}

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

func findWikiRepoCommit(ctx *context.APIContext) (*git.Repository, *git.Commit, error) {
	wikiRepo, err := git.OpenRepository(ctx.Repo.Repository.WikiPath())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
		return nil, nil, err
	}

	commit, err := wikiRepo.GetBranchCommit("master")
	if err != nil {
		return wikiRepo, nil, err
	}
	return wikiRepo, commit, nil
}

// wikiContentsByEntry returns the contents of the wiki page referenced by the
// given tree entry. Writes to ctx if an error occurs.
func wikiContentsByEntry(ctx *context.APIContext, entry *git.TreeEntry) []byte {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "Blob.Data", err)
		return nil
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ReadAll", err)
		return nil
	}
	return content
}

// wikiContentsByName returns the contents of a wiki page, along with a boolean
// indicating whether the page exists. Writes to ctx if an error occurs.
func wikiContentsByName(ctx *context.APIContext, commit *git.Commit, wikiName string) ([]byte, *git.TreeEntry, string, bool) {
	pageFilename := wiki_service.NameToFilename(wikiName)
	entry, err := findEntryForFile(commit, pageFilename)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "findEntryForFile", err)
		return nil, nil, "", false
	} else if entry == nil {
		return nil, nil, "", true
	}
	return wikiContentsByEntry(ctx, entry), entry, pageFilename, false
}
