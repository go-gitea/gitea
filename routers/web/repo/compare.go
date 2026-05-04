// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	gocontext "context"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	issues_model "code.gitea.io/gitea/models/issues"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/charset"
	csv_module "code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/fileicon"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	git_service "code.gitea.io/gitea/services/git"
	"code.gitea.io/gitea/services/gitdiff"
	user_service "code.gitea.io/gitea/services/user"
)

const (
	tplCompare     templates.TplName = "repo/diff/compare"
	tplBlobExcerpt templates.TplName = "repo/diff/blob_excerpt"
	tplDiffBox     templates.TplName = "repo/diff/box"
)

// setCompareContext sets context data.
func setCompareContext(ctx *context.Context, before, head *git.Commit, headOwner, headName string) {
	ctx.Data["BeforeCommit"] = before
	ctx.Data["HeadCommit"] = head

	ctx.Data["GetBlobByPathForCommit"] = func(commit *git.Commit, path string) *git.Blob {
		if commit == nil {
			return nil
		}

		blob, err := commit.GetBlobByPath(path)
		if err != nil {
			return nil
		}
		return blob
	}

	ctx.Data["GetSniffedTypeForBlob"] = func(blob *git.Blob) typesniffer.SniffedType {
		st := typesniffer.SniffedType{}

		if blob == nil {
			return st
		}

		st, err := blob.GuessContentType()
		if err != nil {
			log.Error("GuessContentType failed: %v", err)
			return st
		}
		return st
	}

	setPathsCompareContext(ctx, before, head, headOwner, headName)
	setImageCompareContext(ctx)
	setCsvCompareContext(ctx)
}

// SourceCommitURL creates a relative URL for a commit in the given repository
func SourceCommitURL(owner, name string, commit *git.Commit) string {
	return setting.AppSubURL + "/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/src/commit/" + url.PathEscape(commit.ID.String())
}

// RawCommitURL creates a relative URL for the raw commit in the given repository
func RawCommitURL(owner, name string, commit *git.Commit) string {
	return setting.AppSubURL + "/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/raw/commit/" + url.PathEscape(commit.ID.String())
}

// setPathsCompareContext sets context data for source and raw paths
func setPathsCompareContext(ctx *context.Context, base, head *git.Commit, headOwner, headName string) {
	ctx.Data["SourcePath"] = SourceCommitURL(headOwner, headName, head)
	ctx.Data["RawPath"] = RawCommitURL(headOwner, headName, head)
	if base != nil {
		ctx.Data["BeforeSourcePath"] = SourceCommitURL(headOwner, headName, base)
		ctx.Data["BeforeRawPath"] = RawCommitURL(headOwner, headName, base)
	}
}

// setImageCompareContext sets context data that is required by image compare template
func setImageCompareContext(ctx *context.Context) {
	ctx.Data["IsSniffedTypeAnImage"] = func(st typesniffer.SniffedType) bool {
		return st.IsImage() && (setting.UI.SVG.Enabled || !st.IsSvgImage())
	}
}

// setCsvCompareContext sets context data that is required by the CSV compare template
func setCsvCompareContext(ctx *context.Context) {
	ctx.Data["IsCsvFile"] = func(diffFile *gitdiff.DiffFile) bool {
		extension := strings.ToLower(filepath.Ext(diffFile.Name))
		return extension == ".csv" || extension == ".tsv"
	}

	type CsvDiffResult struct {
		Sections []*gitdiff.TableDiffSection
		Error    string
	}

	ctx.Data["CreateCsvDiff"] = func(diffFile *gitdiff.DiffFile, baseBlob, headBlob *git.Blob) CsvDiffResult {
		if diffFile == nil {
			return CsvDiffResult{nil, ""}
		}

		errTooLarge := errors.New(ctx.Locale.TrString("repo.error.csv.too_large"))

		csvReaderFromCommit := func(ctx *markup.RenderContext, blob *git.Blob) (*csv.Reader, io.Closer, error) {
			if blob == nil {
				// It's ok for blob to be nil (file added or deleted)
				return nil, nil, nil
			}

			if setting.UI.CSV.MaxFileSize != 0 && setting.UI.CSV.MaxFileSize < blob.Size() {
				return nil, nil, errTooLarge
			}

			reader, err := blob.DataAsync()
			if err != nil {
				return nil, nil, err
			}
			var closer io.Closer = reader
			csvReader, err := csv_module.CreateReaderAndDetermineDelimiter(ctx, charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}))
			return csvReader, closer, err
		}

		baseReader, baseBlobCloser, err := csvReaderFromCommit(markup.NewRenderContext(ctx).WithRelativePath(diffFile.OldName), baseBlob)
		if baseBlobCloser != nil {
			defer baseBlobCloser.Close()
		}
		if err != nil {
			if err == errTooLarge {
				return CsvDiffResult{nil, err.Error()}
			}
			log.Error("error whilst creating csv.Reader from file %s in base commit %s in %s: %v", diffFile.Name, baseBlob.ID.String(), ctx.Repo.Repository.Name, err)
			return CsvDiffResult{nil, "unable to load file"}
		}

		headReader, headBlobCloser, err := csvReaderFromCommit(markup.NewRenderContext(ctx).WithRelativePath(diffFile.Name), headBlob)
		if headBlobCloser != nil {
			defer headBlobCloser.Close()
		}
		if err != nil {
			if err == errTooLarge {
				return CsvDiffResult{nil, err.Error()}
			}
			log.Error("error whilst creating csv.Reader from file %s in head commit %s in %s: %v", diffFile.Name, headBlob.ID.String(), ctx.Repo.Repository.Name, err)
			return CsvDiffResult{nil, "unable to load file"}
		}

		sections, err := gitdiff.CreateCsvDiff(diffFile, baseReader, headReader)
		if err != nil {
			errMessage, err := csv_module.FormatError(err, ctx.Locale)
			if err != nil {
				log.Error("CreateCsvDiff FormatError failed: %v", err)
				return CsvDiffResult{nil, "unknown csv diff error"}
			}
			return CsvDiffResult{nil, errMessage}
		}
		return CsvDiffResult{sections, ""}
	}
}

type comparePageInfoType struct {
	compareInfo      *git_service.CompareInfo
	nothingToCompare bool
	allowCreatePull  bool
}

func newComparePageInfo() *comparePageInfoType {
	return &comparePageInfoType{}
}

// parseCompareInfo parse compare info between two commit for preparing comparing references
func (cpi *comparePageInfoType) parseCompareInfo(ctx *context.Context) error {
	baseRepo := ctx.Repo.Repository
	fileOnly := ctx.FormBool("file-only")

	// 1 Parse compare router param
	compareReq := common.ParseCompareRouterParam(ctx.PathParam("*"))

	// remove the check when we support compare with carets
	if compareReq.BaseOriRefSuffix != "" {
		return util.NewInvalidArgumentErrorf("unsupported comparison syntax: ref with suffix")
	}

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

	baseRef := ctx.Repo.GitRepo.UnstableGuessRefByShortName(baseRefName)
	if baseRef == "" {
		return util.NewNotExistErrorf("no base ref: %s", baseRefName)
	}
	headGitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, headRepo)
	if err != nil {
		return err
	}

	headRef := headGitRepo.UnstableGuessRefByShortName(headRefName)
	if headRef == "" {
		return util.NewNotExistErrorf("no head ref: %s", headRefName)
	}

	ctx.Data["BaseName"] = baseRepo.OwnerName
	ctx.Data["BaseBranch"] = baseRef.ShortName() // for legacy templates
	ctx.Data["HeadUser"] = headOwner
	ctx.Data["HeadBranch"] = headRef.ShortName() // for legacy templates
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
	var rootRepo *repo_model.Repository
	if baseRepo.IsFork {
		err = baseRepo.GetBaseRepo(ctx)
		if err != nil && !repo_model.IsErrRepoNotExist(err) {
			return err
		} else if err == nil {
			rootRepo = baseRepo.BaseRepo
		}
	}

	// 2. Now if the current user is not the owner of the baseRepo,
	// check if they have a fork of the base repo and offer that as
	// "OwnForkRepo"
	var ownForkRepo *repo_model.Repository
	if ctx.Doer != nil && baseRepo.OwnerID != ctx.Doer.ID {
		repo := repo_model.GetForkedRepo(ctx, ctx.Doer.ID, baseRepo.ID)
		if repo != nil {
			ownForkRepo = repo
			ctx.Data["OwnForkRepo"] = ownForkRepo
		}
	}

	ctx.Data["HeadRepo"] = headRepo
	ctx.Data["BaseCompareRepo"] = ctx.Repo.Repository

	// If we have a rootRepo, and it's different from:
	// 1. the computed base
	// 2. the computed head
	// then get the branches of it
	if rootRepo != nil &&
		rootRepo.ID != headRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		canRead := access_model.CheckRepoUnitUser(ctx, rootRepo, ctx.Doer, unit.TypeCode)
		if canRead {
			ctx.Data["RootRepo"] = rootRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, rootRepo)
				if err != nil {
					return err
				}
				ctx.Data["RootRepoBranches"] = branches
				ctx.Data["RootRepoTags"] = tags
			}
		}
	}

	// If we have a ownForkRepo, and it's different from:
	// 1. The computed base
	// 2. The computed head
	// 3. The rootRepo (if we have one)
	// then get the branches from it.
	if ownForkRepo != nil &&
		ownForkRepo.ID != headRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		canRead := access_model.CheckRepoUnitUser(ctx, ownForkRepo, ctx.Doer, unit.TypeCode)
		if canRead {
			ctx.Data["OwnForkRepo"] = ownForkRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, ownForkRepo)
				if err != nil {
					return err
				}
				ctx.Data["OwnForkRepoBranches"] = branches
				ctx.Data["OwnForkRepoTags"] = tags
			}
		}
	}

	compareInfo, err := git_service.GetCompareInfo(ctx, baseRepo, headRepo, headGitRepo, baseRef, headRef, compareReq.DirectComparison(), fileOnly)
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
	if useFirstCommitAsTitle {
		// the "commits" are from "ShowPrettyFormatLogToList", which is ordered from newest to oldest, here take the oldest one
		c := commits[len(commits)-1]
		title = strings.TrimSpace(c.UserCommit.Summary())
	} else {
		title = autoTitleFromBranchName(ci.HeadRef.ShortName())
	}

	if len(commits) == 1 {
		// FIXME: GIT-COMMIT-MESSAGE-ENCODING: try to convert the encoding for commit message explicitly, ideally it should be done by a git commit struct method
		c := commits[0]
		_, content, _ = strings.Cut(strings.TrimSpace(c.UserCommit.CommitMessage), "\n")
		content = strings.TrimSpace(content)
		content = string(charset.ToUTF8([]byte(content), charset.ConvertOpts{}))
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

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if len(files) == 2 || len(files) == 1 {
		maxLines, maxFiles = -1, -1
	}

	fileOnly := ctx.FormBool("file-only")

	diff, err := gitdiff.GetDiffForRender(ctx, ci.HeadRepo.Link(), ci.HeadGitRepo,
		&gitdiff.DiffOptions{
			BeforeCommitID:     beforeCommitID,
			AfterCommitID:      headCommitID,
			SkipTo:             ctx.FormString("skip-to"),
			MaxLines:           maxLines,
			MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
			MaxFiles:           maxFiles,
			WhitespaceBehavior: whitespaceBehavior,
			DirectComparison:   ci.DirectComparison(),
		}, ctx.FormStrings("files")...)
	if err != nil {
		ctx.ServerError("GetDiff", err)
		return
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(ctx, ci.HeadRepo, ci.HeadGitRepo, beforeCommitID, headCommitID)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return
	}
	ctx.Data["DiffShortStat"] = diffShortStat
	ctx.Data["Diff"] = diff
	ctx.Data["DiffBlobExcerptData"] = &gitdiff.DiffBlobExcerptData{
		BaseLink:      ci.HeadRepo.Link() + "/blob_excerpt",
		DiffStyle:     GetDiffViewStyle(ctx),
		AfterCommitID: headCommitID,
	}
	ctx.Data["DiffNotAvailable"] = diffShortStat.NumFiles == 0

	if !fileOnly {
		diffTree, err := gitdiff.GetDiffTree(ctx, ci.HeadGitRepo, false, beforeCommitID, headCommitID)
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

	headCommit, err := ci.HeadGitRepo.GetCommit(headCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}

	baseGitRepo := ctx.Repo.GitRepo

	beforeCommit, err := baseGitRepo.GetCommit(beforeCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}

	commits, err := processGitCommits(ctx, ci.Commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return
	}
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	ctx.Data["title"], ctx.Data["content"] = prepareNewPullRequestTitleContent(ci, commits, setting.Repository.PullRequest.DefaultTitleSource)

	setCompareContext(ctx, beforeCommit, headCommit, ci.HeadRepo.OwnerName, repo.Name)
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

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	comparePageInfo := newComparePageInfo()
	err := comparePageInfo.parseCompareInfo(ctx)
	if errors.Is(err, util.ErrNotExist) || errors.Is(err, util.ErrInvalidArgument) {
		ctx.NotFound(nil)
		return
	} else if err != nil {
		ctx.ServerError("ParseCompareInfo", err)
		return
	}
	ci := comparePageInfo.compareInfo
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

	fileOnly := ctx.FormBool("file-only")
	if fileOnly {
		ctx.HTML(http.StatusOK, tplDiffBox)
		return
	}

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

// attachCommentsToLines attaches comments to their corresponding diff lines
func attachCommentsToLines(section *gitdiff.DiffSection, lineComments map[int64][]*issues_model.Comment) {
	for _, line := range section.Lines {
		if comments, ok := lineComments[int64(line.LeftIdx*-1)]; ok {
			line.Comments = append(line.Comments, comments...)
		}
		if comments, ok := lineComments[int64(line.RightIdx)]; ok {
			line.Comments = append(line.Comments, comments...)
		}
		sort.SliceStable(line.Comments, func(i, j int) bool {
			return line.Comments[i].CreatedUnix < line.Comments[j].CreatedUnix
		})
	}
}

// attachHiddenCommentIDs calculates and attaches hidden comment IDs to expand buttons
func attachHiddenCommentIDs(section *gitdiff.DiffSection, lineComments map[int64][]*issues_model.Comment) {
	for _, line := range section.Lines {
		gitdiff.FillHiddenCommentIDsForDiffLine(line, lineComments)
	}
}

// ExcerptBlob render blob excerpt contents
func ExcerptBlob(ctx *context.Context) {
	commitID := ctx.PathParam("sha")
	opts := gitdiff.BlobExcerptOptions{
		LastLeft:      ctx.FormInt("last_left"),
		LastRight:     ctx.FormInt("last_right"),
		LeftIndex:     ctx.FormInt("left"),
		RightIndex:    ctx.FormInt("right"),
		LeftHunkSize:  ctx.FormInt("left_hunk_size"),
		RightHunkSize: ctx.FormInt("right_hunk_size"),
		Direction:     ctx.FormString("direction"),
		Language:      ctx.FormString("filelang"),
	}
	filePath := ctx.FormString("path")
	gitRepo := ctx.Repo.GitRepo

	diffBlobExcerptData := &gitdiff.DiffBlobExcerptData{
		BaseLink:      ctx.Repo.RepoLink + "/blob_excerpt",
		DiffStyle:     GetDiffViewStyle(ctx),
		AfterCommitID: commitID,
	}

	if ctx.Data["PageIsWiki"] == true {
		var err error
		gitRepo, err = gitrepo.RepositoryFromRequestContextOrOpen(ctx, ctx.Repo.Repository.WikiStorageRepo())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		diffBlobExcerptData.BaseLink = ctx.Repo.RepoLink + "/wiki/blob_excerpt"
	}

	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return
	}
	blob, err := commit.Tree.GetBlobByPath(filePath)
	if err != nil {
		ctx.ServerError("GetBlobByPath", err)
		return
	}
	reader, err := blob.DataAsync()
	if err != nil {
		ctx.ServerError("DataAsync", err)
		return
	}
	defer reader.Close()

	section, err := gitdiff.BuildBlobExcerptDiffSection(filePath, reader, opts)
	if err != nil {
		ctx.ServerError("BuildBlobExcerptDiffSection", err)
		return
	}

	diffBlobExcerptData.PullIssueIndex = ctx.FormInt64("pull_issue_index")
	if diffBlobExcerptData.PullIssueIndex > 0 {
		if !ctx.Repo.Permission.CanRead(unit.TypePullRequests) {
			ctx.NotFound(nil)
			return
		}

		issue, err := issues_model.GetIssueByIndex(ctx, ctx.Repo.Repository.ID, diffBlobExcerptData.PullIssueIndex)
		if err != nil {
			log.Error("GetIssueByIndex error: %v", err)
		} else if issue.IsPull {
			// FIXME: DIFF-CONVERSATION-DATA: the following data assignment is fragile
			ctx.Data["Issue"] = issue
			ctx.Data["CanBlockUser"] = func(blocker, blockee *user_model.User) bool {
				return user_service.CanBlockUser(ctx, ctx.Doer, blocker, blockee)
			}
			// and "diff/comment_form.tmpl" (reply comment) needs them
			ctx.Data["PageIsPullFiles"] = true
			ctx.Data["AfterCommitID"] = diffBlobExcerptData.AfterCommitID

			allComments, err := issues_model.FetchCodeComments(ctx, issue, ctx.Doer, ctx.FormBool("show_outdated"))
			if err != nil {
				log.Error("FetchCodeComments error: %v", err)
			} else {
				if lineComments, ok := allComments[filePath]; ok {
					attachCommentsToLines(section, lineComments)
					attachHiddenCommentIDs(section, lineComments)
				}
			}
		}
	}

	ctx.Data["section"] = section
	ctx.Data["FileNameHash"] = git.HashFilePathForWebUI(filePath)
	ctx.Data["DiffBlobExcerptData"] = diffBlobExcerptData

	ctx.HTML(http.StatusOK, tplBlobExcerpt)
}
