// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitgraph"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/gitdiff"
	git_service "code.gitea.io/gitea/services/repository"
)

const (
	tplCommits    base.TplName = "repo/commits"
	tplGraph      base.TplName = "repo/graph"
	tplGraphDiv   base.TplName = "repo/graph/div"
	tplCommitPage base.TplName = "repo/commit_page"
)

// RefCommits render commits page
func RefCommits(ctx *context.Context) {
	switch {
	case len(ctx.Repo.TreePath) == 0:
		Commits(ctx)
	case ctx.Repo.TreePath == "search":
		SearchCommits(ctx)
	default:
		FileHistory(ctx)
	}
}

// Commits render branch's commits
func Commits(ctx *context.Context) {
	ctx.Data["PageIsCommits"] = true
	if ctx.Repo.Commit == nil {
		ctx.NotFound("Commit not found", nil)
		return
	}
	ctx.Data["PageIsViewCode"] = true

	commitsCount, err := ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	pageSize := ctx.FormInt("limit")
	if pageSize <= 0 {
		pageSize = setting.Git.CommitsRangeSize
	}

	// Both `git log branchName` and `git log commitId` work.
	commits, err := ctx.Repo.Commit.CommitsByRange(page, pageSize, "")
	if err != nil {
		ctx.ServerError("CommitsByRange", err)
		return
	}
	ctx.Data["Commits"] = git_model.ConvertFromGitCommit(ctx, commits, ctx.Repo.Repository)

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commitsCount

	pager := context.NewPagination(int(commitsCount), pageSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplCommits)
}

// Graph render commit graph - show commits from all branches.
func Graph(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.commit_graph")
	ctx.Data["PageIsCommits"] = true
	ctx.Data["PageIsViewCode"] = true
	mode := strings.ToLower(ctx.FormTrim("mode"))
	if mode != "monochrome" {
		mode = "color"
	}
	ctx.Data["Mode"] = mode
	hidePRRefs := ctx.FormBool("hide-pr-refs")
	ctx.Data["HidePRRefs"] = hidePRRefs
	branches := ctx.FormStrings("branch")
	realBranches := make([]string, len(branches))
	copy(realBranches, branches)
	for i, branch := range realBranches {
		if strings.HasPrefix(branch, "--") {
			realBranches[i] = git.BranchPrefix + branch
		}
	}
	ctx.Data["SelectedBranches"] = realBranches
	files := ctx.FormStrings("file")

	commitsCount, err := ctx.Repo.GetCommitsCount()
	if err != nil {
		ctx.ServerError("GetCommitsCount", err)
		return
	}

	graphCommitsCount, err := ctx.Repo.GetCommitGraphsCount(ctx, hidePRRefs, realBranches, files)
	if err != nil {
		log.Warn("GetCommitGraphsCount error for generate graph exclude prs: %t branches: %s in %-v, Will Ignore branches and try again. Underlying Error: %v", hidePRRefs, branches, ctx.Repo.Repository, err)
		realBranches = []string{}
		branches = []string{}
		graphCommitsCount, err = ctx.Repo.GetCommitGraphsCount(ctx, hidePRRefs, realBranches, files)
		if err != nil {
			ctx.ServerError("GetCommitGraphsCount", err)
			return
		}
	}

	page := ctx.FormInt("page")

	graph, err := gitgraph.GetCommitGraph(ctx.Repo.GitRepo, page, 0, hidePRRefs, realBranches, files)
	if err != nil {
		ctx.ServerError("GetCommitGraph", err)
		return
	}

	if err := graph.LoadAndProcessCommits(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo); err != nil {
		ctx.ServerError("LoadAndProcessCommits", err)
		return
	}

	ctx.Data["Graph"] = graph

	gitRefs, err := ctx.Repo.GitRepo.GetRefs()
	if err != nil {
		ctx.ServerError("GitRepo.GetRefs", err)
		return
	}

	ctx.Data["AllRefs"] = gitRefs

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commitsCount

	paginator := context.NewPagination(int(graphCommitsCount), setting.UI.GraphMaxCommitNum, page, 5)
	paginator.AddParam(ctx, "mode", "Mode")
	paginator.AddParam(ctx, "hide-pr-refs", "HidePRRefs")
	for _, branch := range branches {
		paginator.AddParamString("branch", branch)
	}
	for _, file := range files {
		paginator.AddParamString("file", file)
	}
	ctx.Data["Page"] = paginator
	if ctx.FormBool("div-only") {
		ctx.HTML(http.StatusOK, tplGraphDiv)
		return
	}

	ctx.HTML(http.StatusOK, tplGraph)
}

// SearchCommits render commits filtered by keyword
func SearchCommits(ctx *context.Context) {
	ctx.Data["PageIsCommits"] = true
	ctx.Data["PageIsViewCode"] = true

	query := ctx.FormTrim("q")
	if len(query) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/commits/" + ctx.Repo.BranchNameSubURL())
		return
	}

	all := ctx.FormBool("all")
	opts := git.NewSearchCommitsOptions(query, all)
	commits, err := ctx.Repo.Commit.SearchCommits(opts)
	if err != nil {
		ctx.ServerError("SearchCommits", err)
		return
	}
	ctx.Data["CommitCount"] = len(commits)
	ctx.Data["Commits"] = git_model.ConvertFromGitCommit(ctx, commits, ctx.Repo.Repository)

	ctx.Data["Keyword"] = query
	if all {
		ctx.Data["All"] = "checked"
	}
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.HTML(http.StatusOK, tplCommits)
}

// FileHistory show a file's reversions
func FileHistory(ctx *context.Context) {
	ctx.Data["IsRepoToolbarCommits"] = true

	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		Commits(ctx)
		return
	}

	commitsCount, err := ctx.Repo.GitRepo.FileCommitsCount(ctx.Repo.RefName, fileName)
	if err != nil {
		ctx.ServerError("FileCommitsCount", err)
		return
	} else if commitsCount == 0 {
		ctx.NotFound("FileCommitsCount", nil)
		return
	}

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	commits, err := ctx.Repo.GitRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: ctx.Repo.RefName,
			File:     fileName,
			Page:     page,
		})
	if err != nil {
		ctx.ServerError("CommitsByFileAndRange", err)
		return
	}
	ctx.Data["Commits"] = git_model.ConvertFromGitCommit(ctx, commits, ctx.Repo.Repository)

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["FileName"] = fileName
	ctx.Data["CommitCount"] = commitsCount

	pager := context.NewPagination(int(commitsCount), setting.Git.CommitsRangeSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplCommits)
}

func LoadBranchesAndTags(ctx *context.Context) {
	response, err := git_service.LoadBranchesAndTags(ctx, ctx.Repo, ctx.Params("sha"))
	if err == nil {
		ctx.JSON(http.StatusOK, response)
		return
	}
	ctx.NotFoundOrServerError(fmt.Sprintf("could not load branches and tags the commit %s belongs to", ctx.Params("sha")), git.IsErrNotExist, err)
}

// Diff show different from current commit to previous commit
func Diff(ctx *context.Context) {
	ctx.Data["PageIsDiff"] = true

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.Params(":sha")
	var (
		gitRepo *git.Repository
		err     error
	)

	if ctx.Data["PageIsWiki"] != nil {
		gitRepo, err = git.OpenRepository(ctx, ctx.Repo.Repository.WikiPath())
		if err != nil {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
			return
		}
		defer gitRepo.Close()
	} else {
		gitRepo = ctx.Repo.GitRepo
	}

	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("Repo.GitRepo.GetCommit", err)
		} else {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
		}
		return
	}
	if len(commitID) != git.SHAFullLength {
		commitID = commit.ID.String()
	}

	fileOnly := ctx.FormBool("file-only")
	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if fileOnly && (len(files) == 2 || len(files) == 1) {
		maxLines, maxFiles = -1, -1
	}

	diff, err := gitdiff.GetDiff(ctx, gitRepo, &gitdiff.DiffOptions{
		AfterCommitID:      commitID,
		SkipTo:             ctx.FormString("skip-to"),
		MaxLines:           maxLines,
		MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:           maxFiles,
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)),
	}, files...)
	if err != nil {
		ctx.NotFound("GetDiff", err)
		return
	}

	parents := make([]string, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, err := commit.ParentID(i)
		if err != nil {
			ctx.NotFound("repo.Diff", err)
			return
		}
		parents[i] = sha.String()
	}

	ctx.Data["CommitID"] = commitID
	ctx.Data["AfterCommitID"] = commitID
	ctx.Data["Username"] = userName
	ctx.Data["Reponame"] = repoName

	var parentCommit *git.Commit
	if commit.ParentCount() > 0 {
		parentCommit, err = gitRepo.GetCommit(parents[0])
		if err != nil {
			ctx.NotFound("GetParentCommit", err)
			return
		}
	}
	setCompareContext(ctx, parentCommit, commit, userName, repoName)
	ctx.Data["Title"] = commit.Summary() + " · " + base.ShortSha(commitID)
	ctx.Data["Commit"] = commit
	ctx.Data["Diff"] = diff

	statuses, _, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, commitID, db.ListOptions{ListAll: true})
	if err != nil {
		log.Error("GetLatestCommitStatus: %v", err)
	}

	ctx.Data["CommitStatus"] = git_model.CalcCommitStatus(statuses)
	ctx.Data["CommitStatuses"] = statuses

	verification := asymkey_model.ParseCommitWithSignature(ctx, commit)
	ctx.Data["Verification"] = verification
	ctx.Data["Author"] = user_model.ValidateCommitWithEmail(ctx, commit)
	ctx.Data["Parents"] = parents
	ctx.Data["DiffNotAvailable"] = diff.NumFiles == 0

	if err := asymkey_model.CalculateTrustStatus(verification, ctx.Repo.Repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
		return repo_model.IsOwnerMemberCollaborator(ctx, ctx.Repo.Repository, user.ID)
	}, nil); err != nil {
		ctx.ServerError("CalculateTrustStatus", err)
		return
	}

	note := &git.Note{}
	err = git.GetNote(ctx, ctx.Repo.GitRepo, commitID, note)
	if err == nil {
		ctx.Data["Note"] = string(charset.ToUTF8WithFallback(note.Message))
		ctx.Data["NoteCommit"] = note.Commit
		ctx.Data["NoteAuthor"] = user_model.ValidateCommitWithEmail(ctx, note.Commit)
	}

	ctx.Data["BranchName"], err = commit.GetBranchName()
	if err != nil {
		ctx.ServerError("commit.GetBranchName", err)
		return
	}

	ctx.HTML(http.StatusOK, tplCommitPage)
}

// RawDiff dumps diff results of repository in given commit ID to io.Writer
func RawDiff(ctx *context.Context) {
	var gitRepo *git.Repository
	if ctx.Data["PageIsWiki"] != nil {
		wikiRepo, err := git.OpenRepository(ctx, ctx.Repo.Repository.WikiPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer wikiRepo.Close()
		gitRepo = wikiRepo
	} else {
		gitRepo = ctx.Repo.GitRepo
		if gitRepo == nil {
			ctx.ServerError("GitRepo not open", fmt.Errorf("no open git repo for '%s'", ctx.Repo.Repository.FullName()))
			return
		}
	}
	if err := git.GetRawDiff(
		gitRepo,
		ctx.Params(":sha"),
		git.RawDiffType(ctx.Params(":ext")),
		ctx.Resp,
	); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound("GetRawDiff",
				errors.New("commit "+ctx.Params(":sha")+" does not exist."))
			return
		}
		ctx.ServerError("GetRawDiff", err)
		return
	}
}
