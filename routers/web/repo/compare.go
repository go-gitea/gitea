// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	gocontext "context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/modules/base"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/routers/common"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	git_service "gitea.dev/services/git"
	"gitea.dev/services/gitdiff"
)

const tplCompare templates.TplName = "repo/diff/compare"

// SourceCommitURL creates a relative URL for a commit in the given repository
func SourceCommitURL(owner, name string, commit *git.Commit) string {
	return setting.AppSubURL + "/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/src/commit/" + url.PathEscape(commit.ID.String())
}

func newComparePageInfo() *comparePageInfoType {
	return &comparePageInfoType{}
}

type comparePageInfoType struct {
	compareInfo      *git_service.CompareInfo
	nothingToCompare bool
	allowCreatePull  bool
	rootRepo         *repo_model.Repository // the base repo was forked from it
	ownForkRepo      *repo_model.Repository // the doer's fork of the base repo
	fileOnly         bool                   // only the diff is needed, not the commits
}

// parseCompareInfo parse compare info between two commit for preparing comparing references
func (cpi *comparePageInfoType) parseCompareInfo(ctx *context.Context, compareParam string) error {
	baseRepo := ctx.Repo.Repository

	// 1 Parse compare router param
	compareReq := common.ParseCompareRouterParam(compareParam)

	// 2 get repository and owner for head
	headOwner, headRepo, err := common.GetHeadOwnerAndRepo(ctx, baseRepo, compareReq)
	if err != nil {
		return err
	}

	// 3 permission check
	// base repository's code unit read permission check has been done on web.go
	permBase := ctx.Repo.Permission

	// If we're not merging from the same repo:
	isSameRepo := baseRepo.ID == headRepo.ID
	if !isSameRepo {
		// Assert ctx.Doer has permission to read headRepo's codes
		permHead, err := access_model.GetDoerRepoPermission(ctx, headRepo, ctx.Doer)
		if err != nil {
			return err
		}
		if !permHead.CanRead(unit.TypeCode) {
			return util.NewNotExistErrorf("") // permission: no error message for end users
		}
		ctx.Data["CanWriteToHeadRepo"] = permHead.CanWrite(unit.TypeCode)
	}

	// 4 get base and head refs
	baseRefName := util.IfZero(compareReq.BaseOriRef, baseRepo.GetPullRequestTargetBranch(ctx))
	headRefName := util.IfZero(compareReq.HeadOriRef, headRepo.DefaultBranch)

	baseRef, err := common.ResolveRefWithSuffix(ctx, ctx.Repo.GitRepo, baseRefName, compareReq.BaseOriRefSuffix)
	if err != nil {
		return err
	}
	headGitRepo, err := git.RepositoryFromRequestContextOrOpen(ctx, headRepo)
	if err != nil {
		return err
	}

	headRef, err := common.ResolveRefWithSuffix(ctx, headGitRepo, headRefName, compareReq.HeadOriRefSuffix)
	if err != nil {
		return err
	}

	// for the dropdown links ("{base} ... {head}") in the compare page "compare.tmpl"
	ctx.Data["BaseName"] = baseRepo.OwnerName
	ctx.Data["HeadUser"] = headOwner
	ctx.Data["BaseBranch"] = baseRef.ShortName() // also for "commits_table.tmpl"'s "no_commits" message
	ctx.Data["HeadBranch"] = headRef.ShortName()

	ctx.Data["IsPull"] = true

	context.InitRepoPullRequestCtx(ctx, baseRepo, headRepo)

	// The current base and head repositories and branches may not
	// actually be the intended branches that the user wants to
	// create a pull-request from - but also determining the head
	// repo is difficult.

	// We will want therefore to offer a few repositories to set as
	// our base and head

	// 1. First if the baseRepo is a fork get the "RootRepo" it was
	// forked from
	if baseRepo.IsFork {
		err = baseRepo.GetBaseRepo(ctx)
		if err != nil && !repo_model.IsErrRepoNotExist(err) {
			return err
		} else if err == nil {
			cpi.rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	if ctx.Doer != nil && baseRepo.OwnerID != ctx.Doer.ID {
		cpi.ownForkRepo = repo_model.GetForkedRepo(ctx, ctx.Doer.ID, baseRepo.ID)
		if cpi.ownForkRepo != nil {
			ctx.Data["OwnForkRepo"] = cpi.ownForkRepo
		}
	}

	ctx.Data["HeadRepo"] = headRepo
	ctx.Data["BaseCompareRepo"] = ctx.Repo.Repository

	compareInfo, err := git_service.GetCompareInfo(ctx, baseRepo, headRepo, headGitRepo, baseRef, headRef, compareReq.DirectComparison(), cpi.fileOnly)
	if err != nil {
		return err
	}

	// Treat as pull request if both references are branches
	cpi.allowCreatePull = baseRef.IsBranch() && headRef.IsBranch() && permBase.CanReadIssuesOrPulls(true)
	cpi.allowCreatePull = cpi.allowCreatePull && compareInfo.CompareBase != ""
	cpi.compareInfo = &compareInfo
	return nil
}

// autoTitleFromBranchName humanizes a branch name into a PR title.
func autoTitleFromBranchName(name string) string {
	var buf strings.Builder
	var prevIsSpace bool
	runes := []rune(name)
	for i, r := range runes {
		isSpace := unicode.IsSpace(r)
		if r == '-' || r == '_' || isSpace {
			if !prevIsSpace {
				buf.WriteRune(' ')
			}
			prevIsSpace = true
			continue
		}
		if !prevIsSpace && unicode.IsUpper(r) {
			needSpace := i > 0 && unicode.IsLower(runes[i-1]) || i < len(runes)-1 && unicode.IsLower(runes[i+1])
			if needSpace {
				buf.WriteRune(' ')
			}
		}
		buf.WriteRune(unicode.ToLower(r))
		prevIsSpace = isSpace
	}
	out := strings.TrimSpace(buf.String())
	if out == "" {
		return out
	}
	outRunes := []rune(out)
	outRunes[0] = unicode.ToUpper(outRunes[0])
	return string(outRunes)
}

func prepareNewPullRequestTitleContent(ci *git_service.CompareInfo, commits []*git_model.SignCommitWithStatuses, defaultTitleSource string) (title, content string) {
	useFirstCommitAsTitle := len(commits) == 1 || (defaultTitleSource == setting.RepoPRTitleSourceFirstCommit && len(commits) > 0)
	if defaultTitleSource == setting.RepoPRTitleSourceBranchName {
		title = ci.HeadRef.ShortName()
	} else if useFirstCommitAsTitle {
		// the "commits" are from "ShowPrettyFormatLogToList", which is ordered from newest to oldest, here take the oldest one
		c := commits[len(commits)-1]
		title = c.UserCommit.GitCommit.MessageTitle()
	} else {
		title = autoTitleFromBranchName(ci.HeadRef.ShortName())
	}

	if len(commits) == 1 {
		c := commits[0]
		content = c.GitCommit.MessageBody()
	}

	var titleTrailer string
	// TODO: 255 doesn't seem to be a good limit for title, just keep the old behavior
	title, titleTrailer = util.EllipsisDisplayStringX(title, 255)
	if titleTrailer != "" {
		if content != "" {
			content = titleTrailer + "\n\n" + content
		} else {
			content = titleTrailer + "\n"
		}
	}
	return title, content
}

// prepareCompareDiff renders compare diff page. TODO: need to refactor it and other "compare diff" related functions together
func (cpi *comparePageInfoType) prepareCompareDiff(ctx *context.Context, whitespaceBehavior gitcmd.TrustedCmdArgs) {
	ci := cpi.compareInfo
	if ci.CompareBase == "" {
		cpi.nothingToCompare = true
		return
	}
	repo := ctx.Repo.Repository
	headCommitID := ci.HeadCommitID

	ctx.Data["CommitRepoLink"] = ci.HeadRepo.Link()
	ctx.Data["BeforeCommitID"] = ci.CompareBase
	ctx.Data["AfterCommitID"] = headCommitID

	// follow GitHub's behavior: autofill the form and expand
	newPrFormTitle := ctx.FormTrim("title")
	newPrFormBody := ctx.FormTrim("body")
	ctx.Data["ExpandNewPrForm"] = ctx.FormBool("expand") || ctx.FormBool("quick_pull") || newPrFormTitle != "" || newPrFormBody != ""
	ctx.Data["TitleQuery"] = newPrFormTitle
	ctx.Data["BodyQuery"] = newPrFormBody

	if headCommitID == ci.CompareBase {
		config := repo.MustGetUnit(ctx, unit.TypePullRequests).PullRequestsConfig()
		// if auto-detect manual merge, an empty PR will be closed immediately because it is already on base branch
		supportEmptyPr := !config.AutodetectManualMerge
		acrossRepoPr := !ci.IsSameRef()
		ctx.Data["AllowEmptyPr"] = supportEmptyPr && acrossRepoPr

		cpi.nothingToCompare = true
		return
	}

	beforeCommitID := ci.CompareBase
	diffCommonOptions := gitdiff.DiffCommonOptions{
		BeforeCommitID:     beforeCommitID,
		AfterCommitID:      headCommitID,
		WhitespaceBehavior: whitespaceBehavior,
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, ci.HeadGitRepo, &diffCommonOptions)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return
	}
	ctx.Data["DiffShortStat"] = diffShortStat
	ctx.Data["DiffNotAvailable"] = diffShortStat.NumFiles == 0
	ctx.Data["ShowDiffSummaryInToolbar"] = diffShortStat.NumFiles != 0

	diffTree, err := gitdiff.GetDiffTree(ctx, ci.HeadGitRepo, false, beforeCommitID, headCommitID)
	if err != nil {
		ctx.ServerError("GetDiffTree", err)
		return
	}
	prepareDiffFileTree(ctx, diffTree, nil)

	commits, err := processGitCommits(ctx, ci.Commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	ctx.Data["title"], ctx.Data["content"] = prepareNewPullRequestTitleContent(ci, commits, setting.Repository.PullRequest.DefaultTitleSource)

	prepareDiffViewer(ctx, diffViewerOptions{
		streamPath:     ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ctx.PathParam("*")) + "/diff/stream",
		gitRepoLink:    ci.HeadRepo.Link(),
		beforeCommitID: beforeCommitID,
		afterCommitID:  headCommitID,
	})
}

func getBranchesAndTagsForRepo(ctx gocontext.Context, repo *repo_model.Repository) (branches, tags []string, err error) {
	branches, err = git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID:          repo.ID,
		ListOptions:     db.ListOptionsAll,
		IsDeletedBranch: optional.Some(false),
	})
	if err != nil {
		return nil, nil, err
	}
	tags, err = repo_model.GetTagNamesByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, nil, err
	}
	return branches, tags, nil
}

func parseComparePageInfo(ctx *context.Context, compareParam string, fileOnly bool) *comparePageInfoType {
	comparePageInfo := newComparePageInfo()
	comparePageInfo.fileOnly = fileOnly
	err := comparePageInfo.parseCompareInfo(ctx, compareParam)
	if errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) {
		ctx.NotFound(nil)
		return nil
	} else if err != nil {
		ctx.ServerError("ParseCompareInfo", err)
		return nil
	}
	return comparePageInfo
}

// prepareRefLists loads the branches and tags of the other repos the page offers to compare, if they differ from the base and head
func (cpi *comparePageInfoType) prepareRefLists(ctx *context.Context) error {
	baseRepo, headRepo := cpi.compareInfo.BaseRepo, cpi.compareInfo.HeadRepo
	if rootRepo := cpi.rootRepo; rootRepo != nil && rootRepo.ID != headRepo.ID && rootRepo.ID != baseRepo.ID &&
		access_model.CheckRepoUnitUser(ctx, rootRepo, ctx.Doer, unit.TypeCode) {
		ctx.Data["RootRepo"] = rootRepo
		branches, tags, err := getBranchesAndTagsForRepo(ctx, rootRepo)
		if err != nil {
			return err
		}
		ctx.Data["RootRepoBranches"] = branches
		ctx.Data["RootRepoTags"] = tags
	}
	if ownForkRepo := cpi.ownForkRepo; ownForkRepo != nil && ownForkRepo.ID != headRepo.ID && ownForkRepo.ID != baseRepo.ID &&
		(cpi.rootRepo == nil || ownForkRepo.ID != cpi.rootRepo.ID) && access_model.CheckRepoUnitUser(ctx, ownForkRepo, ctx.Doer, unit.TypeCode) {
		ctx.Data["OwnForkRepo"] = ownForkRepo
		branches, tags, err := getBranchesAndTagsForRepo(ctx, ownForkRepo)
		if err != nil {
			return err
		}
		ctx.Data["OwnForkRepoBranches"] = branches
		ctx.Data["OwnForkRepoTags"] = tags
	}
	return nil
}

// CompareDiffStream streams the diff of the compare page, the base commit is fetched into the head repo again if it was pruned meanwhile
func CompareDiffStream(ctx *context.Context) {
	comparePageInfo := parseComparePageInfo(ctx, ctx.PathParam("basehead"), true)
	if ctx.Written() {
		return
	}
	ci := comparePageInfo.compareInfo
	serveDiffStream(ctx, ci.HeadGitRepo, gitdiff.DiffStreamOptions{
		DiffCommonOptions: gitdiff.DiffCommonOptions{BeforeCommitID: ci.CompareBase, AfterCommitID: ci.HeadCommitID},
		RepoID:            ci.HeadRepo.ID,
		RepoLink:          ci.HeadRepo.Link(),
	}, nil)
}

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	comparePageInfo := parseComparePageInfo(ctx, ctx.PathParam("*"), false)
	if ctx.Written() {
		return
	}
	ci := comparePageInfo.compareInfo
	if err := comparePageInfo.prepareRefLists(ctx); err != nil {
		ctx.ServerError("prepareRefLists", err)
		return
	}
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["CompareInfo"] = ci

	// TODO: need to refactor "prepare compare" related functions together
	comparePageInfo.prepareCompareDiff(ctx, gitdiff.GetWhitespaceFlag(GetWhitespaceBehavior(ctx)))
	if ctx.Written() {
		return
	}

	baseTags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = baseTags

	headBranches, headTags, err := getBranchesAndTagsForRepo(ctx, ci.HeadRepo)
	if err != nil {
		ctx.ServerError("GetBranchesAndTagsForRepo", err)
		return
	}
	ctx.Data["HeadBranches"] = headBranches
	ctx.Data["HeadTags"] = headTags

	// For compare repo branches
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}

	if ci.CompareBase != "" {
		comparePageInfo.prepareCreatePullRequestPage(ctx)
		if ctx.Written() {
			return
		}
	} else {
		ctx.Flash.Error(ctx.Tr("repo.pulls.no_common_history"), true)
		ctx.Data["CommitCount"] = 0
	}
	ctx.Data["PageIsComparePull"] = comparePageInfo.allowCreatePull
	ctx.Data["IsNothingToCompare"] = comparePageInfo.nothingToCompare
	ctx.HTML(http.StatusOK, tplCompare)
}

// DownloadCompareDiff render a comparison's raw unified diff
func DownloadCompareDiff(ctx *context.Context) {
	downloadCompareDiffOrPatch(ctx, false)
}

// DownloadComparePatch render a comparison as a git format-patch
func DownloadComparePatch(ctx *context.Context) {
	downloadCompareDiffOrPatch(ctx, true)
}

func downloadCompareDiffOrPatch(ctx *context.Context, patch bool) {
	// The route captures `basehead` separately so the `.diff`/`.patch` suffix is
	// stripped from the catch-all `*` param parseCompareInfo would otherwise read.
	cpi := newComparePageInfo()
	if err := cpi.parseCompareInfo(ctx, ctx.PathParam("basehead")); err != nil {
		if errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) {
			ctx.NotFound(nil)
		} else {
			ctx.ServerError("ParseCompareInfo", err)
		}
		return
	}
	ci := cpi.compareInfo

	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	compareArg := ci.BaseCommitID + ci.CompareSeparator + ci.HeadCommitID

	var err error
	if patch {
		err = ci.HeadGitRepo.GetPatch(ctx, compareArg, ctx.Resp)
	} else {
		err = ci.HeadGitRepo.GetDiff(ctx, compareArg, ctx.Resp)
	}
	if err != nil {
		ctx.ServerError("DownloadCompareDiffOrPatch", err)
		return
	}
}

func (cpi *comparePageInfoType) prepareCreatePullRequestPage(ctx *context.Context) {
	ci := cpi.compareInfo
	if cpi.allowCreatePull {
		pr, err := issues_model.GetUnmergedPullRequest(ctx, ci.HeadRepo.ID, ctx.Repo.Repository.ID, ci.HeadRef.ShortName(), ci.BaseRef.ShortName(), issues_model.PullRequestFlowGithub)
		if err != nil {
			if !issues_model.IsErrPullRequestNotExist(err) {
				ctx.ServerError("GetUnmergedPullRequest", err)
				return
			}
		} else {
			ctx.Data["HasPullRequest"] = true
			if err := pr.LoadIssue(ctx); err != nil {
				ctx.ServerError("LoadIssue", err)
				return
			}
			ctx.Data["PullRequest"] = pr
			return
		}

		if !cpi.nothingToCompare {
			// Setup information for new form.
			pageMetaData := retrieveRepoIssueMetaData(ctx, ctx.Repo.Repository, nil, true)
			if ctx.Written() {
				return
			}
			_, templateErrs := setTemplateIfExists(ctx, pullRequestTemplateKey, pullRequestTemplateCandidates, pageMetaData)
			if len(templateErrs) > 0 {
				ctx.Flash.Warning(renderErrorOfTemplates(ctx, templateErrs), true)
			}
		}
	}
	beforeCommitID := cpi.compareInfo.CompareBase
	afterCommitID := cpi.compareInfo.HeadCommitID

	ctx.Data["Title"] = "Comparing " + base.ShortSha(beforeCommitID) + ci.CompareSeparator + base.ShortSha(afterCommitID)

	ctx.Data["IsDiffCompare"] = true

	if content, ok := ctx.Data["content"].(string); ok && content != "" {
		// If a template content is set, prepend the "content". In this case that's only
		// applicable if you have one commit to compare and that commit has a message.
		// In that case the commit message will be prepended to the template body.
		if templateContent, ok := ctx.Data[pullRequestTemplateKey].(string); ok && templateContent != "" {
			// Re-use the same key as that's prioritized over the "content" key.
			// Add two new lines between the content to ensure there's always at least
			// one empty line between them.
			ctx.Data[pullRequestTemplateKey] = content + "\n\n" + templateContent
		}

		// When using form fields, also add content to field with id "body".
		if fields, ok := ctx.Data["Fields"].([]*api.IssueFormField); ok {
			for _, field := range fields {
				if field.ID == "body" {
					if fieldValue, ok := field.Attributes["value"].(string); ok && fieldValue != "" {
						field.Attributes["value"] = content + "\n\n" + fieldValue
					} else {
						field.Attributes["value"] = content
					}
				}
			}
		}
	}

	ctx.Data["IsProjectsEnabled"] = ctx.Repo.Permission.CanWrite(unit.TypeProjects)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.Permission.CanWrite(unit.TypePullRequests)

	prConfig := ctx.Repo.Repository.MustGetUnit(ctx, unit.TypePullRequests).PullRequestsConfig()
	ctx.Data["AllowMaintainerEdit"] = prConfig.DefaultAllowMaintainerEdit
}
