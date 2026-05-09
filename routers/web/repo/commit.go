
// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	git_service "code.gitea.io/gitea/services/git"
	"code.gitea.io/gitea/services/gitdiff"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/services/repository/gitgraph"
)

const (
	tplCommits    templates.TplName = "repo/commits"
	tplGraph      templates.TplName = "repo/graph"
	tplGraphDiv   templates.TplName = "repo/graph/div"
	tplCommitPage templates.TplName = "repo/commit_page"
)

// RefCommits render commits page
func RefCommits(ctx *context.Context) {
	switch {
	case len(ctx.Repo.TreePath) == 0:
		Commits(ctx)
		return
	case ctx.Repo.TreePath == "graphs":
		if setting.Repository.EnableGitGraph {
			Graph(ctx)
		} else {
			ctx.NotFound("Graph", nil)
		}
		return
	case strings.HasPrefix(ctx.Repo.TreePath, "graphs/"):
		if setting.Repository.EnableGitGraph {
			GraphDiv(ctx)
		} else {
			ctx.NotFound("GraphDiv", nil)
		}
		return
	case ctx.Repo.TreePath == "search":
		SearchCommits(ctx)
		return
	case strings.HasPrefix(ctx.Repo.TreePath, "commits/"):
		CommitPage(ctx)
		return
	}

	ctx.NotFound("RefCommits", nil)
}

// Commits render branch's commits
func Commits(ctx *context.Context) {
	ctx.Data["PageIsCommits"] = true
	ctx.Data["Title"] = ctx.Tr("repo.commits.commit_history")

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	var (
		commitsCount int64
		commits      []*git.Commit
		err          error
	)

	branchName := ctx.Repo.BranchName
	if len(ctx.Repo.TreePath) > 0 {
		branchName, err = ctx.Repo.Commit.Submodule(ctx.Repo.TreePath)
		if err != nil {
			ctx.ServerError("Submodule", err)
			return
		}
	}

	// Get the commits
	commits, err = ctx.Repo.GitRepo.CommitsByRangeWithSize(page, branchName)
	if err != nil {
		ctx.ServerError("CommitsByRange", err)
		return
	}
	commitsCount, err = ctx.Repo.GitRepo.RevListCount([]string{branchName})
	if err != nil {
		ctx.ServerError("RevListCount", err)
		return
	}

	// Get sign and verify info
	signCommitInfos, err := processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}

	ctx.Data["Commits"] = signCommitInfos
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commitsCount
	ctx.Data["Branch"] = ctx.Repo.BranchName
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["CommitID"] = ctx.Repo.CommitID

	pager := context.NewPagination(int(commitsCount), setting.UI.ExplorePagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplCommits)
}

// Graph render commit graph - show commits from all branches.
func Graph(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.graph")
	ctx.Data["PageIsCommits"] = true
	ctx.Data["PageIsGraph"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 0
	} else {
		page--
	}

	commits, err := gitgraph.GetCommitGraph(ctx, ctx.Repo.GitRepo, page, setting.UI.GraphMaxCommitNum)
	if err != nil {
		ctx.ServerError("GetCommitGraph", err)
		return
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commits.Count
	ctx.Data["Commits"] = commits.Commits

	ctx.HTML(http.StatusOK, tplGraph)
}

// GraphDiv render commit graph div - show commits from all branches.
func GraphDiv(ctx *context.Context) {
	ctx.Data["PageIsCommits"] = true
	ctx.Data["PageIsGraph"] = true

	page := ctx.FormInt("page")
	if page <= 1 {
		page = 0
	} else {
		page--
	}

	commits, err := gitgraph.GetCommitGraph(ctx, ctx.Repo.GitRepo, page, setting.UI.GraphMaxCommitNum)
	if err != nil {
		ctx.ServerError("GetCommitGraph", err)
		return
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commits.Count
	ctx.Data["Commits"] = commits.Commits

	ctx.HTML(http.StatusOK, tplGraphDiv)
}

// SearchCommits render commits filtered by keyword
func SearchCommits(ctx *context.Context) {
	ctx.Data["PageIsCommits"] = true
	ctx.Data["Title"] = ctx.Tr("repo.commits.commit_history")

	keyword := ctx.FormTrim("q")
	if len(keyword) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/commits/" + ctx.Repo.BranchName)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	commits, err := ctx.Repo.GitRepo.SearchCommits(keyword)
	if err != nil {
		ctx.ServerError("SearchCommits", err)
		return
	}

	// Get sign and verify info
	signCommitInfos, err := processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}

	ctx.Data["Commits"] = signCommitInfos
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["Keyword"] = keyword
	ctx.Data["Branch"] = ctx.Repo.BranchName
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["CommitCount"] = len(commits)

	pager := context.NewPagination(len(commits), setting.UI.ExplorePagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplCommits)
}

// FileHistory show a file's reversions
func FileHistory(ctx *context.Context) {
	ctx.Data["IsRepoToolbarCommits"] = true
	ctx.Data["IsRepoToolbarFile"] = true

	fileName := ctx.Repo.TreePath
	if len(fileName) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/commits/" + ctx.Repo.BranchName)
		return
	}

	page := ctx.FormInt("page")
	if page <= 0 {
		page = 1
	}

	// Get the commits
	commits, err := ctx.Repo.GitRepo.FileCommitsByRange(ctx.Repo.BranchName, fileName, page)
	if err != nil {
		ctx.ServerError("FileCommitsByRange", err)
		return
	}

	// Get sign and verify info
	signCommitInfos, err := processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}

	// Get latest commit
	lastCommit := signCommitInfos[0]
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["FileName"] = fileName
	ctx.Data["CommitCount"] = len(commits)
	ctx.Data["Commits"] = signCommitInfos
	ctx.Data["LastCommit"] = lastCommit
	ctx.Data["IsImageFile"] = lastCommit.Tree.IsImageFile(fileName)
	ctx.Data["Branch"] = ctx.Repo.BranchName
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	ctx.Data["RawFileLink"] = ctx.Repo.RepoLink + "/raw/" + ctx.Repo.BranchNameSubURL() + "/" + util.PathEscapeSegments(fileName)

	pager := context.NewPagination(len(commits), setting.UI.ExplorePagingNum, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplCommits)
}

// LoadBranchesAndTags loads branches and tags from the repository
func LoadBranchesAndTags(ctx *context.Context) {
	branches, err := ctx.Repo.Repository.GetBranches()
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	ctx.Data["Branches"] = branches

	tags, err := ctx.Repo.Repository.GetTags(0, 0)
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["Tags"] = tags
}

// Diff show different from current commit to previous commit of the file
func Diff(ctx *context.Context) {
	ctx.Data["PageIsDiff"] = true

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.PathParam("sha")

	diffBlobExcerptData := &gitdiff.DiffBlobExcerptData{
		BaseLink:      ctx.Repo.RepoLink + "/blob_excerpt",
		DiffStyle:     GetDiffViewStyle(ctx),
		AfterCommitID: commitID,
	}
	gitRepo := ctx.Repo.GitRepo
	var gitRepoStore gitrepo.Repository = ctx.Repo.Repository

	if ctx.Data["PageIsWiki"] != nil {
		var err error
		gitRepoStore = ctx.Repo.Repository.WikiStorageRepo()
		gitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, gitRepoStore)
		if err != nil {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
			return
		}
		diffBlobExcerptData.BaseLink = ctx.Repo.RepoLink + "/wiki/blob_excerpt"
	}

	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("Repo.GitRepo.GetCommit", err)
		}
		return
	}
	if len(commitID) != commit.ID.Type().FullLength() {
		commitID = commit.ID.String()
	}

	fileOnly := ctx.FormBool("file-only")
	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if fileOnly && (len(files) == 2 || len(files) == 1) {
		maxLines, maxFiles = -1, -1
	}

	diff, err := gitdiff.GetDiffForRender(ctx, ctx.Repo.RepoLink, gitRepo, &gitdiff.DiffOptions{
		AfterCommitID:      commitID,
		SkipTo:             ctx.FormString("skip-to"),
		MaxLines:           maxLines,
		MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
		MaxFiles:           maxFiles,
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(GetWhitespaceBehavior(ctx)),
	}, files...)
	if err != nil {
		ctx.NotFound(err)
		return
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, gitRepoStore, gitRepo, "", commitID)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return
	}
	ctx.Data["DiffShortStat"] = diffShortStat

	parents := make([]string, commit.ParentCount())
	for i := 0; i < commit.ParentCount(); i++ {
		sha, err := commit.ParentID(i)
		if err != nil {
			ctx.NotFound(err)
			return
		}
		parents[i] = sha.String()
	}

	ctx.Data["CommitID"] = commitID
	ctx.Data["AfterCommitID"] = commitID

	var parentCommit *git.Commit
	var parentCommitID string
	if commit.ParentCount() > 0 {
		parentCommit, err = gitRepo.GetCommit(parents[0])
		if err != nil {
			ctx.NotFound(err)
			return
		}
		parentCommitID = parentCommit.ID.String()
	}
	setCompareContext(ctx, parentCommit, commit, userName, repoName)
	ctx.Data["Title"] = commit.MessageTitle() + " · " + base.ShortSha(commitID)
	ctx.Data["Commit"] = commit
	ctx.Data["Diff"] = diff
	ctx.Data["DiffBlobExcerptData"] = diffBlobExcerptData

	// Load commit comments for diff lines
	if err := repo_model.LoadCommentsForDiffLines(ctx, ctx.Repo.Repository.ID, commitID, diff.Lines); err != nil {
		ctx.ServerError("LoadCommentsForDiffLines", err)
		return
	}

	if !fileOnly {
		diffTree, err := gitdiff.GetDiffTree(ctx, gitRepo, false, parentCommitID, commitID)
		if err != nil {
			ctx.ServerError("GetDiffTree", err)
			return
		}

		renderedIconPool := fileicon.NewRenderedIconPool()
		ctx.PageData["DiffFileTree"] = transformDiffTreeForWeb(renderedIconPool, diffTree, nil)
		ctx.PageData["FolderIcon"] = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolder())
		ctx.PageData["FolderOpenIcon"] = fileicon.RenderEntryIconHTML(renderedIconPool, fileicon.EntryInfoFolderOpen())
		ctx.Data["FileIconPoolHTML"] = renderedIconPool.RenderToHTML()
	}

	statuses, err := git_model.GetLatestCommitStatus(ctx, ctx.Repo.Repository.ID, commitID, db.ListOptionsAll)
	if err != nil {
		log.Error("GetLatestCommitStatus: %v", err)
	}
	if !ctx.Repo.Permission.CanRead(unit_model.TypeActions) {
		git_model.CommitStatusesHideActionsURL(ctx, statuses)
	}

	ctx.Data["CommitStatus"] = git_model.CalcCommitStatus(statuses)
	ctx.Data["CommitStatuses"] = statuses

	verification := asymkey_service.ParseCommitWithSignature(ctx, commit)
	ctx.Data["Verification"] = verification
	ctx.Data["Author"] = user_model.ValidateCommitWithEmail(ctx, commit)
	ctx.Data["Parents"] = parents
	ctx.Data["DiffNotAvailable"] = diffShortStat.NumFiles == 0

	if err := asymkey_model.CalculateTrustStatus(verification, ctx.Repo.Repository.GetTrustModel(), func(user *user_model.User) (bool, error) {
		return repo_model.IsOwnerMemberCollaborator(ctx, ctx.Repo.Repository, user.ID)
	}, nil); err != nil {
		ctx.ServerError("CalculateTrustStatus", err)
		return
	}

	note := &git.Note{}
	err = git.GetNote(ctx, ctx.Repo.GitRepo, commitID, note)
	if err == nil {
		ctx.Data["NoteCommit"] = note.Commit
		ctx.Data["NoteAuthor"] = user_model.ValidateCommitWithEmail(ctx, note.Commit)
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{CurrentRefSubURL: "commit/" + util.PathEscapeSegments(commitID)})
		htmlMessage := template.HTML(template.HTMLEscapeString(string(charset.ToUTF8WithFallback(note.Message, charset.ConvertOpts{}))))
		ctx.Data["NoteRendered"], err = markup.PostProcessCommitMessage(rctx, htmlMessage)
		if err != nil {
			ctx.ServerError("PostProcessCommitMessage", err)
			return
		}
	} else if !git.IsErrNotExist(err) {
		log.Error("GetNote: %v", err)
	}

	pr, _ := issues_model.GetPullRequestByMergedCommit(ctx, ctx.Repo.Repository.ID, commitID)
	if pr != nil {
		ctx.Data["MergedPRIssueNumber"] = pr.Index
	}

	ctx.HTML(http.StatusOK, tplCommitPage)
}

// RawDiff dumps diff results of repository in given commit ID to io.Writer
func RawDiff(ctx *context.Context) {
	commitID := ctx.PathParam("sha")
	gitRepo, err := gitrepo.OpenRepository(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("OpenRepository", err)
		return
	}
	defer gitRepo.Close()

	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommit", err)
		}
		return
	}

	diff, err := commit.Diff(ctx.Query("ignore whitespace"))
	if err != nil {
		ctx.ServerError("Diff", err)
		return
	}
	defer diff.Free()

	ctx.Resp.Header().Set("Content-Type", "text/plain")
	ctx.Resp.WriteHeader(http.StatusOK)
	if err := diff.Write(ctx.Resp, git.RawDiffFormat); err != nil {
		log.Error("Write: %v", err)
	}
}

func processGitCommits(ctx *context.Context, gitCommits []*git.Commit) ([]*git_model.SignCommitWithStatuses, error) {
	commits := make([]*git_model.SignCommitWithStatuses, 0, len(gitCommits))
	for _, commit := range gitCommits {
		signCommit, err := git_model.GetSignCommit(ctx, ctx.Repo.Repository.ID, commit)
		if err != nil {
			return nil, err
		}
		commits = append(commits, signCommit)
	}
	return commits, nil
}

