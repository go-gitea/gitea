// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"html/template"
	"maps"
	"net/http"
	"path"
	"strconv"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	git_service "code.gitea.io/gitea/services/git"
	"code.gitea.io/gitea/services/gitdiff"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/services/repository/gitgraph"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplCommits                templates.TplName = "repo/commits"
	tplGraph                  templates.TplName = "repo/graph"
	tplGraphDiv               templates.TplName = "repo/graph/div"
	tplCommitPage             templates.TplName = "repo/commit_page"
	tplCommitConversation     templates.TplName = "repo/diff/commit_conversation"
	tplCommitCommentReactions templates.TplName = "repo/issue/view_content/commit_reactions"
	tplCommitAttachment       templates.TplName = "repo/issue/view_content/commit_attachments"
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
		ctx.NotFound(nil)
		return
	}
	ctx.Data["PageIsViewCode"] = true

	commitsCount := ctx.Repo.CommitsCount

	page := max(ctx.FormInt("page"), 1)

	pageSize := ctx.FormInt("limit")
	if pageSize <= 0 {
		pageSize = setting.Git.CommitsRangeSize
	}

	// Both `git log branchName` and `git log commitId` work.
	commits, err := ctx.Repo.Commit.CommitsByRange(page, pageSize, "", "", "")
	if err != nil {
		ctx.ServerError("CommitsByRange", err)
		return
	}
	ctx.Data["Commits"], err = processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}
	commitIDs := make([]string, 0, len(commits))
	for _, c := range commits {
		commitIDs = append(commitIDs, c.ID.String())
	}
	commitsTagsMap, err := repo_model.FindTagsByCommitIDs(ctx, ctx.Repo.Repository.ID, commitIDs...)
	if err != nil {
		log.Error("FindTagsByCommitIDs: %v", err)
		ctx.Flash.Error(ctx.Tr("internal_error_skipped", "FindTagsByCommitIDs"))
	} else {
		ctx.Data["CommitsTagsMap"] = commitsTagsMap
	}
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["CommitCount"] = commitsCount

	pager := context.NewPagination(int(commitsCount), pageSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
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

	graphCommitsCount, err := ctx.Repo.GetCommitGraphsCount(ctx, hidePRRefs, realBranches, files)
	if err != nil {
		log.Warn("GetCommitGraphsCount error for generate graph exclude prs: %t branches: %s in %-v, Will Ignore branches and try again. Underlying Error: %v", hidePRRefs, branches, ctx.Repo.Repository, err)
		realBranches = []string{}
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

	divOnly := ctx.FormBool("div-only")
	queryParams := ctx.Req.URL.Query()
	queryParams.Del("div-only")
	paginator := context.NewPagination(int(graphCommitsCount), setting.UI.GraphMaxCommitNum, page, 5)
	paginator.AddParamFromQuery(queryParams)
	ctx.Data["Page"] = paginator
	if divOnly {
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
		ctx.Redirect(ctx.Repo.RepoLink + "/commits/" + ctx.Repo.RefTypeNameSubURL())
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
	ctx.Data["Commits"], err = processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}

	ctx.Data["Keyword"] = query
	if all {
		ctx.Data["All"] = true
	}
	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.HTML(http.StatusOK, tplCommits)
}

// FileHistory show a file's reversions
func FileHistory(ctx *context.Context) {
	if ctx.Repo.TreePath == "" {
		Commits(ctx)
		return
	}

	commitsCount, err := ctx.Repo.GitRepo.FileCommitsCount(ctx.Repo.RefFullName.ShortName(), ctx.Repo.TreePath)
	if err != nil {
		ctx.ServerError("FileCommitsCount", err)
		return
	} else if commitsCount == 0 {
		ctx.NotFound(nil)
		return
	}

	page := max(ctx.FormInt("page"), 1)

	commits, err := ctx.Repo.GitRepo.CommitsByFileAndRange(
		git.CommitsByFileAndRangeOptions{
			Revision: ctx.Repo.RefFullName.ShortName(), // FIXME: legacy code used ShortName
			File:     ctx.Repo.TreePath,
			Page:     page,
		})
	if err != nil {
		ctx.ServerError("CommitsByFileAndRange", err)
		return
	}
	ctx.Data["Commits"], err = processGitCommits(ctx, commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}

	ctx.Data["Username"] = ctx.Repo.Owner.Name
	ctx.Data["Reponame"] = ctx.Repo.Repository.Name
	ctx.Data["FileTreePath"] = ctx.Repo.TreePath
	ctx.Data["CommitCount"] = commitsCount

	pager := context.NewPagination(int(commitsCount), setting.Git.CommitsRangeSize, page, 5)
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplCommits)
}

func LoadBranchesAndTags(ctx *context.Context) {
	response, err := repo_service.LoadBranchesAndTags(ctx, ctx.Repo, ctx.PathParam("sha"))
	if err == nil {
		ctx.JSON(http.StatusOK, response)
		return
	}
	ctx.NotFoundOrServerError(fmt.Sprintf("could not load branches and tags the commit %s belongs to", ctx.PathParam("sha")), git.IsErrNotExist, err)
}

// Diff show different from current commit to previous commit
func Diff(ctx *context.Context) {
	ctx.Data["PageIsDiff"] = true

	userName := ctx.Repo.Owner.Name
	repoName := ctx.Repo.Repository.Name
	commitID := ctx.PathParam("sha")
	var (
		gitRepo *git.Repository
		err     error
	)

	if ctx.Data["PageIsWiki"] != nil {
		gitRepo, err = gitrepo.OpenRepository(ctx, ctx.Repo.Repository.WikiStorageRepo())
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
		WhitespaceBehavior: gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)),
	}, files...)
	if err != nil {
		ctx.NotFound(err)
		return
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(gitRepo, "", commitID)
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
	ctx.Data["Username"] = userName
	ctx.Data["Reponame"] = repoName
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled

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
	ctx.Data["Title"] = commit.Summary() + " · " + base.ShortSha(commitID)
	ctx.Data["Commit"] = commit
	ctx.Data["Diff"] = diff

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
	if !ctx.Repo.CanRead(unit_model.TypeActions) {
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
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{CurrentRefPath: path.Join("commit", util.PathEscapeSegments(commitID))})
		ctx.Data["NoteRendered"], err = markup.PostProcessCommitMessage(rctx, template.HTMLEscapeString(string(charset.ToUTF8WithFallback(note.Message, charset.ConvertOpts{}))))
		if err != nil {
			ctx.ServerError("PostProcessCommitMessage", err)
			return
		}
	}

	pr, _ := issues_model.GetPullRequestByMergedCommit(ctx, ctx.Repo.Repository.ID, commitID)
	if pr != nil {
		ctx.Data["MergedPRIssueNumber"] = pr.Index
	}

	commitComment, err := git_model.GetCommitCommentBySHA(ctx, ctx.Repo.Repository.ID, commitID)
	if err != nil {
		ctx.ServerError("GetCommitCommentBySHA", err)
		return
	}

	if commitComment != nil {
		err := git_service.LoadCommitComments(ctx, diff, commitComment, ctx.Doer)
		if err != nil {
			ctx.ServerError("LoadCommitComments", err)
			return
		}
	}
	upload.AddUploadContext(ctx, "comment")
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}

	ctx.HTML(http.StatusOK, tplCommitPage)
}

// RawDiff dumps diff results of repository in given commit ID to io.Writer
func RawDiff(ctx *context.Context) {
	var gitRepo *git.Repository
	if ctx.Data["PageIsWiki"] != nil {
		wikiRepo, err := gitrepo.OpenRepository(ctx, ctx.Repo.Repository.WikiStorageRepo())
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
		ctx.PathParam("sha"),
		git.RawDiffType(ctx.PathParam("ext")),
		ctx.Resp,
	); err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(errors.New("commit " + ctx.PathParam("sha") + " does not exist."))
			return
		}
		ctx.ServerError("GetRawDiff", err)
		return
	}
}

func processGitCommits(ctx *context.Context, gitCommits []*git.Commit) ([]*git_model.SignCommitWithStatuses, error) {
	commits, err := git_service.ConvertFromGitCommit(ctx, gitCommits, ctx.Repo.Repository)
	if err != nil {
		return nil, err
	}
	if !ctx.Repo.CanRead(unit_model.TypeActions) {
		for _, commit := range commits {
			if commit.Status == nil {
				continue
			}
			commit.Status.HideActionsURL(ctx)
			git_model.CommitStatusesHideActionsURL(ctx, commit.Statuses)
		}
	}
	return commits, nil
}

func RenderCommitCommentForm(ctx *context.Context) {
	CommitSHA := ctx.PathParam("sha")

	ctx.Data["PageIsDiff"] = true
	ctx.Data["CommitSHA"] = CommitSHA
	ctx.Data["AfterCommitID"] = CommitSHA
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")
	ctx.HTML(http.StatusOK, tplNewComment)
}

func CreateCommitComment(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CodeCommentForm)
	if ctx.Written() {
		return
	}

	signedLine := form.Line
	if form.Side == "previous" {
		signedLine *= -1
	}
	replyid := form.Reply

	attachmentsMap := git_model.GetAttachments()

	opts := git_model.CreateCommitCommentOptions{
		RefRepoID:        ctx.Repo.Repository.ID,
		Repo:             ctx.Repo.Repository,
		Doer:             ctx.Doer,
		Comment:          form.Content,
		FileName:         form.TreePath,
		LineNum:          signedLine,
		ReplyToCommentID: replyid,
		CommitSHA:        form.LatestCommitID,
		Attachments:      attachmentsMap,
	}
	commitComment, err := git_service.CreateCommitComment(ctx,
		ctx.Doer,
		ctx.Repo.GitRepo,
		opts,
	)
	if err != nil {
		ctx.ServerError("CreateCommitComment", err)
		return
	}
	git_model.ClearAttachments()
	renderCommitComment(ctx, commitComment, form.Origin, signedLine)
}

func renderCommitComment(ctx *context.Context, commitComment *git_model.CommitComment, origin string, signedLine int64) {
	ctx.Data["PageIsDiff"] = true

	opts := git_model.FindCommitCommentOptions{
		CommitSHA: commitComment.CommitSHA,
		Line:      signedLine,
	}
	comments, err := git_model.FindCommitCommentsByLine(ctx, &opts, commitComment)
	if err != nil {
		ctx.ServerError("FetchCodeCommentsByLine", err)
		return
	}

	if len(comments) == 0 {
		ctx.HTML(http.StatusOK, tplConversationOutdated)
		return
	}

	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled

	upload.AddUploadContext(ctx, "comment")
	ctx.Data["comments"] = comments
	ctx.Data["AfterCommitID"] = commitComment.CommitSHA
	ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
		return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
	}

	switch origin {
	case "diff":
		ctx.HTML(http.StatusOK, tplCommitConversation)
	case "timeline":
		ctx.HTML(http.StatusOK, tplTimelineConversation)
	default:
		ctx.HTTPError(http.StatusBadRequest, "Unknown origin: "+origin)
	}
}

// DeleteCommitComment delete comment of commit
func DeleteCommitComment(ctx *context.Context) {
	commitComment, err := git_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		return
	}
	if !ctx.IsSigned || (ctx.Doer.ID != commitComment.PosterID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if err = git_model.DeleteCommitComment(ctx, commitComment); err != nil {
		ctx.ServerError("DeleteCommitComment", err)
		return
	}

	ctx.Status(http.StatusOK)
}

func UpdateCommitComment(ctx *context.Context) {
	commitComment, err := git_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetCommitCommentByID", err)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != commitComment.PosterID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	newContent := ctx.FormString("content")

	if newContent != commitComment.Comment {
		commitComment.Comment = newContent
		commitComment.ContentVersion++
	}
	files := ctx.FormStrings("files[]")
	filesMap := make(map[string]struct{})
	for _, key := range files {
		filesMap[key] = struct{}{}
	}

	uploadedAttachments := git_model.GetAttachments()

	attachmentMap := make(git_model.AttachmentMap)
	err = json.Unmarshal([]byte(commitComment.Attachments), &attachmentMap)
	if err != nil {
		ctx.ServerError("UpdateCommitComment", err)
		return
	}

	// handle removed files
	for key := range attachmentMap {
		if _, exists := filesMap[key]; !exists {
			delete(attachmentMap, key)
		}
	}
	maps.Copy(attachmentMap, uploadedAttachments)
	err = git_model.UpdateCommitComment(ctx, &attachmentMap, commitComment)
	if err != nil {
		ctx.ServerError("UpdateCommitComment", err)
		return
	}

	var renderedContent template.HTML
	if commitComment.Comment != "" {
		rctx := renderhelper.NewRenderContextRepoComment(ctx, ctx.Repo.Repository, renderhelper.RepoCommentOptions{
			FootnoteContextID: strconv.FormatInt(commitComment.ID, 10),
		})
		renderedContent, err = markdown.RenderString(rctx, commitComment.Comment)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}
	} else {
		contentEmpty := fmt.Sprintf(`<span class="no-content">%s</span>`, ctx.Tr("repo.issues.no_content"))
		renderedContent = template.HTML(contentEmpty)
	}

	attachHTML, err := ctx.RenderToHTML(tplCommitAttachment, map[string]any{
		"Attachments":   attachmentMap,
		"CommitComment": commitComment,
	})
	if err != nil {
		ctx.ServerError("attachmentsHTML.HTMLString", err)
		return
	}

	git_model.ClearAttachments()
	upload.AddUploadContext(ctx, "comment")
	ctx.JSON(http.StatusOK, map[string]any{
		"content":        renderedContent,
		"contentVersion": commitComment.ContentVersion,
		"attachments":    attachHTML,
	})
}

func ChangeCommitCommentReaction(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ReactionForm)
	commitComment, err := git_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.ServerError("GetCommitCommentByID", err)
		return
	}

	action := ctx.PathParam("action")

	if !ctx.IsSigned || (ctx.Doer.ID != commitComment.PosterID) {
		if log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v not the Poster (ID: %d) and cannot read %s in Repo %-v.\n"+
					"User in Repo has Permissions: %-+v",
					ctx.Doer,
					commitComment.PosterID,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Not logged in")
			}
		}

		ctx.HTTPError(http.StatusForbidden)
		return
	}

	switch action {
	case "react":
		err := git_service.CreateCommentReaction(ctx, ctx.Doer, commitComment, form.Content)
		if err != nil {
			log.Info("CreateCommentReaction: %s", err)
			break
		}

	case "unreact":
		if err := git_service.DeleteCommentReaction(ctx, ctx.Doer, commitComment, form.Content); err != nil {
			ctx.ServerError("DeleteCommentReaction", err)
			return
		}

	default:
		ctx.NotFound(nil)
		return
	}

	if len(commitComment.Reactions) == 0 {
		ctx.JSON(http.StatusOK, map[string]any{
			"empty": true,
			"html":  "",
		})
		return
	}
	reactions, _ := commitComment.GroupReactionsByType()
	html, err := ctx.RenderToHTML(tplCommitCommentReactions, map[string]any{
		"ActionURL":     fmt.Sprintf("%s/commit/%s/comments/%d/reactions", ctx.Repo.RepoLink, commitComment.CommitSHA, commitComment.ID),
		"Reactions":     reactions,
		"CommitComment": commitComment,
	})
	if err != nil {
		ctx.ServerError("ChangeCommentReaction.HTMLString", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"html": html,
	})
}

func UploadCommitAttachment(ctx *context.Context) {
	if !ctx.IsSigned {
		ctx.HTTPError(http.StatusForbidden)
		return
	}

	if !setting.Attachment.Enabled {
		ctx.HTTPError(http.StatusNotFound, "attachment is not enabled")
		return
	}

	file, header, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, fmt.Sprintf("FormFile: %v", err))
		return
	}

	validExt := strings.Contains(setting.Attachment.AllowedTypes, path.Ext(header.Filename))
	if !validExt {
		ctx.HTTPError(http.StatusNotFound, "attachment type not valid")
		return
	}
	ctx.Data["PageIsDiff"] = true

	opts := git_model.AttachmentOptions{
		FileName:   header.Filename,
		Size:       header.Size,
		UploaderID: ctx.Doer.ID,
	}
	attachmentUUID, err := git_service.SaveTemporaryAttachment(ctx, file, &opts)
	if err != nil {
		ctx.ServerError("SaveTemporaryAttachment", err)
		return
	}
	git_model.AddAttachment(attachmentUUID, opts)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	ctx.JSON(http.StatusOK, map[string]string{
		"uuid": attachmentUUID,
	})
}

func DeleteCommitAttachment(ctx *context.Context) {
	uuid := ctx.FormString("file")
	attachments := git_model.GetAttachments()
	attachment := attachments[uuid]
	if attachment == nil {
		ctx.HTTPError(http.StatusNotFound)
		return
	}

	if !ctx.IsSigned || (ctx.Doer.ID != attachment.UploaderID) {
		ctx.HTTPError(http.StatusForbidden)
		return
	}
	err := storage.Attachments.Delete(uuid)
	if err != nil {
		ctx.ServerError("delete ", err)
		return
	}

	git_model.RemoveAttachment(uuid)
}

func GetCommitAttachmentByUUID(ctx *context.Context) {
	commitComment, err := git_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		return
	}

	repository, err := repo_model.GetRepositoryByID(ctx, commitComment.RefRepoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}

	attachmentMap := make(git_model.AttachmentMap)
	err = json.Unmarshal([]byte(commitComment.Attachments), &attachmentMap)
	if err != nil {
		ctx.ServerError("GetCommitAttachmentByUUID", err)
		return
	}
	uuid := ctx.PathParam("uuid")

	attachment := attachmentMap[uuid]
	if attachment == nil {
		ctx.HTTPError(http.StatusNotFound)
		return
	}
	if repository == nil {
		if !(ctx.IsSigned && attachment.UploaderID == ctx.Doer.ID) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	} else {
		_, err := access_model.GetUserRepoPermission(ctx, repository, ctx.Doer)
		if err != nil {
			ctx.HTTPError(http.StatusInternalServerError, "GetUserRepoPermission", err.Error())
			return
		}
	}

	fr, err := storage.Attachments.Open(uuid)
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()

	common.ServeContentByReadSeeker(ctx.Base, attachment.FileName, util.ToPointer(commitComment.CreatedUnix.AsTime()), fr)
}

func GetCommitAttachments(ctx *context.Context) {
	commitComment, err := git_model.GetCommitCommentByID(ctx, ctx.Repo.Repository.ID, ctx.PathParamInt64("id"))
	if err != nil {
		return
	}

	repository, err := repo_model.GetRepositoryByID(ctx, commitComment.RefRepoID)
	if err != nil {
		ctx.ServerError("GetRepositoryByID", err)
		return
	}
	if repository == nil {
		if !(ctx.IsSigned) {
			ctx.HTTPError(http.StatusNotFound)
			return
		}
	}

	git_model.ClearAttachments()

	type AttachmentItem struct {
		Name string `json:"name"`
		UUID string `json:"uuid"`
		Size int64  `json:"size"`
	}

	attachmentMap := make(git_model.AttachmentMap)
	err = json.Unmarshal([]byte(commitComment.Attachments), &attachmentMap)
	if err != nil {
		ctx.ServerError("GetCommitAttachments", err)
		return
	}

	attachmentItems := []*AttachmentItem{}
	for key, value := range attachmentMap {
		item := &AttachmentItem{
			Name: value.FileName, Size: value.Size, UUID: key,
		}
		attachmentItems = append(attachmentItems, item)
	}

	ctx.JSON(http.StatusOK, attachmentItems)
}
