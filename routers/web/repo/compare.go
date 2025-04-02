// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bufio"
	gocontext "context"
	"encoding/csv"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

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
	"code.gitea.io/gitea/modules/git"
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
	"code.gitea.io/gitea/services/gitdiff"
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

			csvReader, err := csv_module.CreateReaderAndDetermineDelimiter(ctx, charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{}))
			return csvReader, reader, err
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

// ParseCompareInfo parse compare info between two commit for preparing comparing references
// Permission check for base repository's code read should be checked before invoking this function
func ParseCompareInfo(ctx *context.Context) *common.CompareInfo {
	fileOnly := ctx.FormBool("file-only")
	pathParam := ctx.PathParam("*")
	baseRepo := ctx.Repo.Repository

	ci, err := common.ParseComparePathParams(ctx, pathParam, baseRepo, ctx.Repo.GitRepo)
	if err != nil {
		switch {
		case user_model.IsErrUserNotExist(err):
			ctx.NotFound(nil)
		case repo_model.IsErrRepoNotExist(err):
			ctx.NotFound(nil)
		case errors.Is(err, util.ErrInvalidArgument):
			ctx.NotFound(nil)
		case git.IsErrNotExist(err):
			ctx.NotFound(nil)
		default:
			ctx.ServerError("ParseComparePathParams", err)
		}
		return nil
	}

	// remove the check when we support compare with carets
	if ci.CaretTimes > 0 {
		ctx.NotFound(nil)
		return nil
	}

	if ci.BaseOriRef == ctx.Repo.GetObjectFormat().EmptyObjectID().String() {
		if ci.IsSameRepo() {
			ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadOriRef))
		} else {
			ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadRepo.FullName()) + ":" + util.PathEscapeSegments(ci.HeadOriRef))
		}
		return nil
	}

	// If we're not merging from the same repo:
	if !ci.IsSameRepo() {
		// Assert ctx.Doer has permission to read headRepo's codes
		permHead, err := access_model.GetUserRepoPermission(ctx, ci.HeadRepo, ctx.Doer)
		if err != nil {
			ctx.ServerError("GetUserRepoPermission", err)
			return nil
		}
		if !permHead.CanRead(unit.TypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					ctx.Doer,
					ci.HeadRepo,
					permHead)
			}
			ctx.NotFound(nil)
			return nil
		}
		ctx.Data["CanWriteToHeadRepo"] = permHead.CanWrite(unit.TypeCode)
	}

	ctx.Data["PageIsComparePull"] = ci.IsPull() && ctx.Repo.CanReadIssuesOrPulls(true)
	ctx.Data["BaseName"] = baseRepo.OwnerName
	ctx.Data["BaseBranch"] = ci.BaseOriRef
	ctx.Data["HeadUser"] = ci.HeadUser
	ctx.Data["HeadBranch"] = ci.HeadOriRef
	ctx.Repo.PullRequest.SameRepo = ci.IsSameRepo()

	ctx.Data["BaseIsCommit"] = ci.IsBaseCommit
	ctx.Data["BaseIsBranch"] = ci.BaseFullRef.IsBranch()
	ctx.Data["BaseIsTag"] = ci.BaseFullRef.IsTag()
	ctx.Data["IsPull"] = true

	ctx.Data["HeadRepo"] = ci.HeadRepo
	ctx.Data["BaseCompareRepo"] = ctx.Repo.Repository
	ctx.Data["HeadIsCommit"] = ci.IsHeadCommit
	ctx.Data["HeadIsBranch"] = ci.HeadFullRef.IsBranch()
	ctx.Data["HeadIsTag"] = ci.HeadFullRef.IsTag()

	ci.CompareInfo, err = ci.HeadGitRepo.GetCompareInfo(baseRepo.RepoPath(), ci.BaseFullRef.String(), ci.HeadFullRef.String(), ci.DirectComparison(), fileOnly)
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return nil
	}
	if ci.DirectComparison() {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.BaseCommitID
	} else {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.MergeBase
	}

	return ci
}

// PrepareCompareDiff renders compare diff page
func PrepareCompareDiff(
	ctx *context.Context,
	ci *common.CompareInfo,
	whitespaceBehavior git.TrustedCmdArgs,
) (nothingToCompare bool) {
	repo := ctx.Repo.Repository
	headCommitID := ci.CompareInfo.HeadCommitID

	ctx.Data["CommitRepoLink"] = ci.HeadRepo.Link()
	ctx.Data["AfterCommitID"] = headCommitID
	ctx.Data["ExpandNewPrForm"] = ctx.FormBool("expand")

	if (headCommitID == ci.CompareInfo.MergeBase && !ci.DirectComparison()) ||
		headCommitID == ci.CompareInfo.BaseCommitID {
		ctx.Data["IsNothingToCompare"] = true
		if unit, err := repo.GetUnit(ctx, unit.TypePullRequests); err == nil {
			config := unit.PullRequestsConfig()

			if !config.AutodetectManualMerge {
				allowEmptyPr := !(ci.BaseOriRef == ci.HeadOriRef && ctx.Repo.Repository.Name == ci.HeadRepo.Name)
				ctx.Data["AllowEmptyPr"] = allowEmptyPr

				return !allowEmptyPr
			}

			ctx.Data["AllowEmptyPr"] = false
		}
		return true
	}

	beforeCommitID := ci.CompareInfo.MergeBase
	if ci.DirectComparison() {
		beforeCommitID = ci.CompareInfo.BaseCommitID
	}

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if len(files) == 2 || len(files) == 1 {
		maxLines, maxFiles = -1, -1
	}

	fileOnly := ctx.FormBool("file-only")

	diff, err := gitdiff.GetDiffForRender(ctx, ci.HeadGitRepo,
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
		return false
	}
	diffShortStat, err := gitdiff.GetDiffShortStat(ci.HeadGitRepo, beforeCommitID, headCommitID)
	if err != nil {
		ctx.ServerError("GetDiffShortStat", err)
		return false
	}
	ctx.Data["DiffShortStat"] = diffShortStat
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diffShortStat.NumFiles == 0

	if !fileOnly {
		diffTree, err := gitdiff.GetDiffTree(ctx, ci.HeadGitRepo, false, beforeCommitID, headCommitID)
		if err != nil {
			ctx.ServerError("GetDiffTree", err)
			return false
		}

		ctx.PageData["DiffFiles"] = transformDiffTreeForUI(diffTree, nil)
	}

	headCommit, err := ci.HeadGitRepo.GetCommit(headCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	baseGitRepo := ctx.Repo.GitRepo

	beforeCommit, err := baseGitRepo.GetCommit(beforeCommitID)
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return false
	}

	commits, err := processGitCommits(ctx, ci.CompareInfo.Commits)
	if err != nil {
		ctx.ServerError("processGitCommits", err)
		return false
	}
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	title := ci.HeadOriRef
	if len(commits) == 1 {
		c := commits[0]
		title = strings.TrimSpace(c.UserCommit.Summary())

		body := strings.Split(strings.TrimSpace(c.UserCommit.Message()), "\n")
		if len(body) > 1 {
			ctx.Data["content"] = strings.Join(body[1:], "\n")
		}
	}

	if len(title) > 255 {
		var trailer string
		title, trailer = util.EllipsisDisplayStringX(title, 255)
		if len(trailer) > 0 {
			if ctx.Data["content"] != nil {
				ctx.Data["content"] = fmt.Sprintf("%s\n\n%s", trailer, ctx.Data["content"])
			} else {
				ctx.Data["content"] = trailer + "\n"
			}
		}
	}

	ctx.Data["title"] = title
	ctx.Data["Username"] = ci.HeadUser.Name
	ctx.Data["Reponame"] = ci.HeadRepo.Name

	setCompareContext(ctx, beforeCommit, headCommit, ci.HeadUser.Name, repo.Name)

	return false
}

func getBranchesAndTagsForRepo(ctx gocontext.Context, repo *repo_model.Repository) ([]string, []string, error) {
	branches, err := git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID:          repo.ID,
		ListOptions:     db.ListOptionsAll,
		IsDeletedBranch: optional.Some(false),
	})
	if err != nil {
		return nil, nil, err
	}
	// always put default branch on the top if it exists
	if slices.Contains(branches, repo.DefaultBranch) {
		branches = util.SliceRemoveAll(branches, repo.DefaultBranch)
		branches = append([]string{repo.DefaultBranch}, branches...)
	}

	tags, err := repo_model.GetTagNamesByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, nil, err
	}

	return branches, tags, nil
}

func prepareCompareRepoBranchesTagsDropdowns(ctx *context.Context, ci *common.CompareInfo) {
	baseRepo := ctx.Repo.Repository
	// For compare repo branches
	baseBranches, baseTags, err := getBranchesAndTagsForRepo(ctx, baseRepo)
	if err != nil {
		ctx.ServerError("getBranchesAndTagsForRepo", err)
		return
	}

	ctx.Data["Branches"] = baseBranches
	ctx.Data["Tags"] = baseTags

	if ci.IsSameRepo() {
		ctx.Data["HeadBranches"] = baseBranches
		ctx.Data["HeadTags"] = baseTags
	} else {
		headBranches, headTags, err := getBranchesAndTagsForRepo(ctx, ci.HeadRepo)
		if err != nil {
			ctx.ServerError("getBranchesAndTagsForRepo", err)
			return
		}
		ctx.Data["HeadBranches"] = headBranches
		ctx.Data["HeadTags"] = headTags
	}

	rootRepo, ownForkRepo, err := ci.LoadRootRepoAndOwnForkRepo(ctx, baseRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("LoadRootRepoAndOwnForkRepo", err)
		return
	}

	if rootRepo != nil &&
		rootRepo.ID != ci.HeadRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		canRead := access_model.CheckRepoUnitUser(ctx, rootRepo, ctx.Doer, unit.TypeCode)
		if canRead {
			ctx.Data["RootRepo"] = rootRepo
			branches, tags, err := getBranchesAndTagsForRepo(ctx, rootRepo)
			if err != nil {
				ctx.ServerError("GetBranchesForRepo", err)
				return
			}
			ctx.Data["RootRepoBranches"] = branches
			ctx.Data["RootRepoTags"] = tags
		}
	}

	if ownForkRepo != nil &&
		ownForkRepo.ID != ci.HeadRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		ctx.Data["OwnForkRepo"] = ownForkRepo
		branches, tags, err := getBranchesAndTagsForRepo(ctx, ownForkRepo)
		if err != nil {
			ctx.ServerError("GetBranchesForRepo", err)
			return
		}
		ctx.Data["OwnForkRepoBranches"] = branches
		ctx.Data["OwnForkRepoTags"] = tags
	}
}

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	ci := ParseCompareInfo(ctx)
	defer func() {
		if ci != nil {
			ci.Close()
		}
	}()
	if ctx.Written() {
		return
	}

	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["DirectComparison"] = ci.DirectComparison
	ctx.Data["OtherCompareSeparator"] = ".."
	ctx.Data["CompareSeparator"] = "..."
	if ci.DirectComparison() {
		ctx.Data["CompareSeparator"] = ".."
		ctx.Data["OtherCompareSeparator"] = "..."
	}

	nothingToCompare := PrepareCompareDiff(ctx, ci, gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)))
	if ctx.Written() {
		return
	}

	fileOnly := ctx.FormBool("file-only")
	if fileOnly {
		ctx.HTML(http.StatusOK, tplDiffBox)
		return
	}

	prepareCompareRepoBranchesTagsDropdowns(ctx, ci)
	if ctx.Written() {
		return
	}

	if ctx.Data["PageIsComparePull"] == true {
		pr, err := issues_model.GetUnmergedPullRequest(ctx, ci.HeadRepo.ID, ctx.Repo.Repository.ID, ci.HeadOriRef, ci.BaseOriRef, issues_model.PullRequestFlowGithub)
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
			ctx.HTML(http.StatusOK, tplCompareDiff)
			return
		}

		if !nothingToCompare {
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
	beforeCommitID := ctx.Data["BeforeCommitID"].(string)
	afterCommitID := ctx.Data["AfterCommitID"].(string)
	separator := ci.CompareDots()

	ctx.Data["Title"] = "Comparing " + base.ShortSha(beforeCommitID) + separator + base.ShortSha(afterCommitID)

	ctx.Data["IsDiffCompare"] = true

	if content, ok := ctx.Data["content"].(string); ok && content != "" {
		// If a template content is set, prepend the "content". In this case that's only
		// applicable if you have one commit to compare and that commit has a message.
		// In that case the commit message will be prepend to the template body.
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

	ctx.Data["IsProjectsEnabled"] = ctx.Repo.CanWrite(unit.TypeProjects)
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "comment")

	ctx.Data["HasIssuesOrPullsWritePermission"] = ctx.Repo.CanWrite(unit.TypePullRequests)

	if unit, err := ctx.Repo.Repository.GetUnit(ctx, unit.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		ctx.Data["AllowMaintainerEdit"] = config.DefaultAllowMaintainerEdit
	} else {
		ctx.Data["AllowMaintainerEdit"] = false
	}

	ctx.HTML(http.StatusOK, tplCompare)
}

// ExcerptBlob render blob excerpt contents
func ExcerptBlob(ctx *context.Context) {
	commitID := ctx.PathParam("sha")
	lastLeft := ctx.FormInt("last_left")
	lastRight := ctx.FormInt("last_right")
	idxLeft := ctx.FormInt("left")
	idxRight := ctx.FormInt("right")
	leftHunkSize := ctx.FormInt("left_hunk_size")
	rightHunkSize := ctx.FormInt("right_hunk_size")
	anchor := ctx.FormString("anchor")
	direction := ctx.FormString("direction")
	filePath := ctx.FormString("path")
	gitRepo := ctx.Repo.GitRepo
	if ctx.Data["PageIsWiki"] == true {
		var err error
		gitRepo, err = gitrepo.OpenRepository(ctx, ctx.Repo.Repository.WikiStorageRepo())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer gitRepo.Close()
	}
	chunkSize := gitdiff.BlobExcerptChunkSize
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, "GetCommit")
		return
	}
	section := &gitdiff.DiffSection{
		FileName: filePath,
	}
	if direction == "up" && (idxLeft-lastLeft) > chunkSize {
		idxLeft -= chunkSize
		idxRight -= chunkSize
		leftHunkSize += chunkSize
		rightHunkSize += chunkSize
		section.Lines, err = getExcerptLines(commit, filePath, idxLeft-1, idxRight-1, chunkSize)
	} else if direction == "down" && (idxLeft-lastLeft) > chunkSize {
		section.Lines, err = getExcerptLines(commit, filePath, lastLeft, lastRight, chunkSize)
		lastLeft += chunkSize
		lastRight += chunkSize
	} else {
		offset := -1
		if direction == "down" {
			offset = 0
		}
		section.Lines, err = getExcerptLines(commit, filePath, lastLeft, lastRight, idxRight-lastRight+offset)
		leftHunkSize = 0
		rightHunkSize = 0
		idxLeft = lastLeft
		idxRight = lastRight
	}
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, "getExcerptLines")
		return
	}
	if idxRight > lastRight {
		lineText := " "
		if rightHunkSize > 0 || leftHunkSize > 0 {
			lineText = fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", idxLeft, leftHunkSize, idxRight, rightHunkSize)
		}
		lineText = html.EscapeString(lineText)
		lineSection := &gitdiff.DiffLine{
			Type:    gitdiff.DiffLineSection,
			Content: lineText,
			SectionInfo: &gitdiff.DiffLineSectionInfo{
				Path:          filePath,
				LastLeftIdx:   lastLeft,
				LastRightIdx:  lastRight,
				LeftIdx:       idxLeft,
				RightIdx:      idxRight,
				LeftHunkSize:  leftHunkSize,
				RightHunkSize: rightHunkSize,
			},
		}
		switch direction {
		case "up":
			section.Lines = append([]*gitdiff.DiffLine{lineSection}, section.Lines...)
		case "down":
			section.Lines = append(section.Lines, lineSection)
		}
	}
	ctx.Data["section"] = section
	ctx.Data["FileNameHash"] = git.HashFilePathForWebUI(filePath)
	ctx.Data["AfterCommitID"] = commitID
	ctx.Data["Anchor"] = anchor
	ctx.HTML(http.StatusOK, tplBlobExcerpt)
}

func getExcerptLines(commit *git.Commit, filePath string, idxLeft, idxRight, chunkSize int) ([]*gitdiff.DiffLine, error) {
	blob, err := commit.Tree.GetBlobByPath(filePath)
	if err != nil {
		return nil, err
	}
	reader, err := blob.DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	var diffLines []*gitdiff.DiffLine
	for line := 0; line < idxRight+chunkSize; line++ {
		if ok := scanner.Scan(); !ok {
			break
		}
		if line < idxRight {
			continue
		}
		lineText := scanner.Text()
		diffLine := &gitdiff.DiffLine{
			LeftIdx:  idxLeft + (line - idxRight) + 1,
			RightIdx: line + 1,
			Type:     gitdiff.DiffLinePlain,
			Content:  " " + lineText,
		}
		diffLines = append(diffLines, diffLine)
	}
	if err = scanner.Err(); err != nil {
		return nil, fmt.Errorf("getExcerptLines scan: %w", err)
	}
	return diffLines, nil
}
