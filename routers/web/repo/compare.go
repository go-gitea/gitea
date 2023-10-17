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
	"code.gitea.io/gitea/modules/context"
	csv_module "code.gitea.io/gitea/modules/csv"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/gitdiff"
)

const (
	tplCompare     base.TplName = "repo/diff/compare"
	tplBlobExcerpt base.TplName = "repo/diff/blob_excerpt"
	tplDiffBox     base.TplName = "repo/diff/box"
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

		errTooLarge := errors.New(ctx.Locale.Tr("repo.error.csv.too_large"))

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

			csvReader, err := csv_module.CreateReaderAndDetermineDelimiter(ctx, charset.ToUTF8WithFallbackReader(reader))
			return csvReader, reader, err
		}

		baseReader, baseBlobCloser, err := csvReaderFromCommit(&markup.RenderContext{Ctx: ctx, RelativePath: diffFile.OldName}, baseBlob)
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

		headReader, headBlobCloser, err := csvReaderFromCommit(&markup.RenderContext{Ctx: ctx, RelativePath: diffFile.Name}, headBlob)
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

// CompareInfo represents the collected results from ParseCompareInfo
type CompareInfo struct {
	HeadUser         *user_model.User
	HeadRepo         *repo_model.Repository
	HeadGitRepo      *git.Repository
	CompareInfo      *git.CompareInfo
	BaseBranch       string
	HeadBranch       string
	DirectComparison bool
}

// ParseCompareInfo parse compare info between two commit for preparing comparing references
func ParseCompareInfo(ctx *context.Context) *CompareInfo {
	baseRepo := ctx.Repo.Repository
	ci := &CompareInfo{}

	fileOnly := ctx.FormBool("file-only")

	// Get compared branches information
	// A full compare url is of the form:
	//
	// 1. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headBranch}
	// 2. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}:{:headBranch}
	// 3. /{:baseOwner}/{:baseRepoName}/compare/{:baseBranch}...{:headOwner}/{:headRepoName}:{:headBranch}
	// 4. /{:baseOwner}/{:baseRepoName}/compare/{:headBranch}
	// 5. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}:{:headBranch}
	// 6. /{:baseOwner}/{:baseRepoName}/compare/{:headOwner}/{:headRepoName}:{:headBranch}
	//
	// Here we obtain the infoPath "{:baseBranch}...[{:headOwner}/{:headRepoName}:]{:headBranch}" as ctx.Params("*")
	// with the :baseRepo in ctx.Repo.
	//
	// Note: Generally :headRepoName is not provided here - we are only passed :headOwner.
	//
	// How do we determine the :headRepo?
	//
	// 1. If :headOwner is not set then the :headRepo = :baseRepo
	// 2. If :headOwner is set - then look for the fork of :baseRepo owned by :headOwner
	// 3. But... :baseRepo could be a fork of :headOwner's repo - so check that
	// 4. Now, :baseRepo and :headRepos could be forks of the same repo - so check that
	//
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	var (
		isSameRepo bool
		infoPath   string
		err        error
	)

	infoPath = ctx.Params("*")
	var infos []string
	if infoPath == "" {
		infos = []string{baseRepo.DefaultBranch, baseRepo.DefaultBranch}
	} else {
		infos = strings.SplitN(infoPath, "...", 2)
		if len(infos) != 2 {
			if infos = strings.SplitN(infoPath, "..", 2); len(infos) == 2 {
				ci.DirectComparison = true
				ctx.Data["PageIsComparePull"] = false
			} else {
				infos = []string{baseRepo.DefaultBranch, infoPath}
			}
		}
	}

	ctx.Data["BaseName"] = baseRepo.OwnerName
	ci.BaseBranch = infos[0]
	ctx.Data["BaseBranch"] = ci.BaseBranch

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		ci.HeadUser = ctx.Repo.Owner
		ci.HeadBranch = headInfos[0]
	} else if len(headInfos) == 2 {
		headInfosSplit := strings.Split(headInfos[0], "/")
		if len(headInfosSplit) == 1 {
			ci.HeadUser, err = user_model.GetUserByName(ctx, headInfos[0])
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil
			}
			ci.HeadBranch = headInfos[1]
			isSameRepo = ci.HeadUser.ID == ctx.Repo.Owner.ID
			if isSameRepo {
				ci.HeadRepo = baseRepo
			}
		} else {
			ci.HeadRepo, err = repo_model.GetRepositoryByOwnerAndName(ctx, headInfosSplit[0], headInfosSplit[1])
			if err != nil {
				if repo_model.IsErrRepoNotExist(err) {
					ctx.NotFound("GetRepositoryByOwnerAndName", nil)
				} else {
					ctx.ServerError("GetRepositoryByOwnerAndName", err)
				}
				return nil
			}
			if err := ci.HeadRepo.LoadOwner(ctx); err != nil {
				if user_model.IsErrUserNotExist(err) {
					ctx.NotFound("GetUserByName", nil)
				} else {
					ctx.ServerError("GetUserByName", err)
				}
				return nil
			}
			ci.HeadBranch = headInfos[1]
			ci.HeadUser = ci.HeadRepo.Owner
			isSameRepo = ci.HeadRepo.ID == ctx.Repo.Repository.ID
		}
	} else {
		ctx.NotFound("CompareAndPullRequest", nil)
		return nil
	}
	ctx.Data["HeadUser"] = ci.HeadUser
	ctx.Data["HeadBranch"] = ci.HeadBranch
	ctx.Repo.PullRequest.SameRepo = isSameRepo

	// Check if base branch is valid.
	baseIsCommit := ctx.Repo.GitRepo.IsCommitExist(ci.BaseBranch)
	baseIsBranch := ctx.Repo.GitRepo.IsBranchExist(ci.BaseBranch)
	baseIsTag := ctx.Repo.GitRepo.IsTagExist(ci.BaseBranch)
	if !baseIsCommit && !baseIsBranch && !baseIsTag {
		// Check if baseBranch is short sha commit hash
		if baseCommit, _ := ctx.Repo.GitRepo.GetCommit(ci.BaseBranch); baseCommit != nil {
			ci.BaseBranch = baseCommit.ID.String()
			ctx.Data["BaseBranch"] = ci.BaseBranch
			baseIsCommit = true
		} else if ci.BaseBranch == git.EmptySHA {
			if isSameRepo {
				ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadBranch))
			} else {
				ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + util.PathEscapeSegments(ci.HeadRepo.FullName()) + ":" + util.PathEscapeSegments(ci.HeadBranch))
			}
			return nil
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil
		}
	}
	ctx.Data["BaseIsCommit"] = baseIsCommit
	ctx.Data["BaseIsBranch"] = baseIsBranch
	ctx.Data["BaseIsTag"] = baseIsTag
	ctx.Data["IsPull"] = true

	// Now we have the repository that represents the base

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
		if err != nil {
			if !repo_model.IsErrRepoNotExist(err) {
				ctx.ServerError("Unable to find root repo", err)
				return nil
			}
		} else {
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

	has := ci.HeadRepo != nil
	// 3. If the base is a forked from "RootRepo" and the owner of
	// the "RootRepo" is the :headUser - set headRepo to that
	if !has && rootRepo != nil && rootRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = rootRepo
		has = true
	}

	// 4. If the ctx.Doer has their own fork of the baseRepo and the headUser is the ctx.Doer
	// set the headRepo to the ownFork
	if !has && ownForkRepo != nil && ownForkRepo.OwnerID == ci.HeadUser.ID {
		ci.HeadRepo = ownForkRepo
		has = true
	}

	// 5. If the headOwner has a fork of the baseRepo - use that
	if !has {
		ci.HeadRepo = repo_model.GetForkedRepo(ctx, ci.HeadUser.ID, baseRepo.ID)
		has = ci.HeadRepo != nil
	}

	// 6. If the baseRepo is a fork and the headUser has a fork of that use that
	if !has && baseRepo.IsFork {
		ci.HeadRepo = repo_model.GetForkedRepo(ctx, ci.HeadUser.ID, baseRepo.ForkID)
		has = ci.HeadRepo != nil
	}

	// 7. Otherwise if we're not the same repo and haven't found a repo give up
	if !isSameRepo && !has {
		ctx.Data["PageIsComparePull"] = false
	}

	// 8. Finally open the git repo
	if isSameRepo {
		ci.HeadRepo = ctx.Repo.Repository
		ci.HeadGitRepo = ctx.Repo.GitRepo
	} else if has {
		ci.HeadGitRepo, err = git.OpenRepository(ctx, ci.HeadRepo.RepoPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return nil
		}
		defer ci.HeadGitRepo.Close()
	} else {
		ctx.NotFound("ParseCompareInfo", nil)
		return nil
	}

	ctx.Data["HeadRepo"] = ci.HeadRepo
	ctx.Data["BaseCompareRepo"] = ctx.Repo.Repository

	// Now we need to assert that the ctx.Doer has permission to read
	// the baseRepo's code and pulls
	// (NOT headRepo's)
	permBase, err := access_model.GetUserRepoPermission(ctx, baseRepo, ctx.Doer)
	if err != nil {
		ctx.ServerError("GetUserRepoPermission", err)
		return nil
	}
	if !permBase.CanRead(unit.TypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.Doer,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil
	}

	// If we're not merging from the same repo:
	if !isSameRepo {
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
			ctx.NotFound("ParseCompareInfo", nil)
			return nil
		}
		ctx.Data["CanWriteToHeadRepo"] = permHead.CanWrite(unit.TypeCode)
	}

	// If we have a rootRepo and it's different from:
	// 1. the computed base
	// 2. the computed head
	// then get the branches of it
	if rootRepo != nil &&
		rootRepo.ID != ci.HeadRepo.ID &&
		rootRepo.ID != baseRepo.ID {
		canRead := access_model.CheckRepoUnitUser(ctx, rootRepo, ctx.Doer, unit.TypeCode)
		if canRead {
			ctx.Data["RootRepo"] = rootRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, rootRepo)
				if err != nil {
					ctx.ServerError("GetBranchesForRepo", err)
					return nil
				}

				ctx.Data["RootRepoBranches"] = branches
				ctx.Data["RootRepoTags"] = tags
			}
		}
	}

	// If we have a ownForkRepo and it's different from:
	// 1. The computed base
	// 2. The computed head
	// 3. The rootRepo (if we have one)
	// then get the branches from it.
	if ownForkRepo != nil &&
		ownForkRepo.ID != ci.HeadRepo.ID &&
		ownForkRepo.ID != baseRepo.ID &&
		(rootRepo == nil || ownForkRepo.ID != rootRepo.ID) {
		canRead := access_model.CheckRepoUnitUser(ctx, ownForkRepo, ctx.Doer, unit.TypeCode)
		if canRead {
			ctx.Data["OwnForkRepo"] = ownForkRepo
			if !fileOnly {
				branches, tags, err := getBranchesAndTagsForRepo(ctx, ownForkRepo)
				if err != nil {
					ctx.ServerError("GetBranchesForRepo", err)
					return nil
				}
				ctx.Data["OwnForkRepoBranches"] = branches
				ctx.Data["OwnForkRepoTags"] = tags
			}
		}
	}

	// Check if head branch is valid.
	headIsCommit := ci.HeadGitRepo.IsCommitExist(ci.HeadBranch)
	headIsBranch := ci.HeadGitRepo.IsBranchExist(ci.HeadBranch)
	headIsTag := ci.HeadGitRepo.IsTagExist(ci.HeadBranch)
	if !headIsCommit && !headIsBranch && !headIsTag {
		// Check if headBranch is short sha commit hash
		if headCommit, _ := ci.HeadGitRepo.GetCommit(ci.HeadBranch); headCommit != nil {
			ci.HeadBranch = headCommit.ID.String()
			ctx.Data["HeadBranch"] = ci.HeadBranch
			headIsCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return nil
		}
	}
	ctx.Data["HeadIsCommit"] = headIsCommit
	ctx.Data["HeadIsBranch"] = headIsBranch
	ctx.Data["HeadIsTag"] = headIsTag

	// Treat as pull request if both references are branches
	if ctx.Data["PageIsComparePull"] == nil {
		ctx.Data["PageIsComparePull"] = headIsBranch && baseIsBranch
	}

	if ctx.Data["PageIsComparePull"] == true && !permBase.CanReadIssuesOrPulls(true) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot create/read pull requests in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.Doer,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareInfo", nil)
		return nil
	}

	baseBranchRef := ci.BaseBranch
	if baseIsBranch {
		baseBranchRef = git.BranchPrefix + ci.BaseBranch
	} else if baseIsTag {
		baseBranchRef = git.TagPrefix + ci.BaseBranch
	}
	headBranchRef := ci.HeadBranch
	if headIsBranch {
		headBranchRef = git.BranchPrefix + ci.HeadBranch
	} else if headIsTag {
		headBranchRef = git.TagPrefix + ci.HeadBranch
	}

	ci.CompareInfo, err = ci.HeadGitRepo.GetCompareInfo(baseRepo.RepoPath(), baseBranchRef, headBranchRef, ci.DirectComparison, fileOnly)
	if err != nil {
		ctx.ServerError("GetCompareInfo", err)
		return nil
	}
	if ci.DirectComparison {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.BaseCommitID
	} else {
		ctx.Data["BeforeCommitID"] = ci.CompareInfo.MergeBase
	}

	return ci
}

// PrepareCompareDiff renders compare diff page
func PrepareCompareDiff(
	ctx *context.Context,
	ci *CompareInfo,
	whitespaceBehavior git.TrustedCmdArgs,
) bool {
	var (
		repo  = ctx.Repo.Repository
		err   error
		title string
	)

	// Get diff information.
	ctx.Data["CommitRepoLink"] = ci.HeadRepo.Link()

	headCommitID := ci.CompareInfo.HeadCommitID

	ctx.Data["AfterCommitID"] = headCommitID

	if (headCommitID == ci.CompareInfo.MergeBase && !ci.DirectComparison) ||
		headCommitID == ci.CompareInfo.BaseCommitID {
		ctx.Data["IsNothingToCompare"] = true
		if unit, err := repo.GetUnit(ctx, unit.TypePullRequests); err == nil {
			config := unit.PullRequestsConfig()

			if !config.AutodetectManualMerge {
				allowEmptyPr := !(ci.BaseBranch == ci.HeadBranch && ctx.Repo.Repository.Name == ci.HeadRepo.Name)
				ctx.Data["AllowEmptyPr"] = allowEmptyPr

				return !allowEmptyPr
			}

			ctx.Data["AllowEmptyPr"] = false
		}
		return true
	}

	beforeCommitID := ci.CompareInfo.MergeBase
	if ci.DirectComparison {
		beforeCommitID = ci.CompareInfo.BaseCommitID
	}

	maxLines, maxFiles := setting.Git.MaxGitDiffLines, setting.Git.MaxGitDiffFiles
	files := ctx.FormStrings("files")
	if len(files) == 2 || len(files) == 1 {
		maxLines, maxFiles = -1, -1
	}

	diff, err := gitdiff.GetDiff(ctx, ci.HeadGitRepo,
		&gitdiff.DiffOptions{
			BeforeCommitID:     beforeCommitID,
			AfterCommitID:      headCommitID,
			SkipTo:             ctx.FormString("skip-to"),
			MaxLines:           maxLines,
			MaxLineCharacters:  setting.Git.MaxGitDiffLineCharacters,
			MaxFiles:           maxFiles,
			WhitespaceBehavior: whitespaceBehavior,
			DirectComparison:   ci.DirectComparison,
		}, ctx.FormStrings("files")...)
	if err != nil {
		ctx.ServerError("GetDiffRangeWithWhitespaceBehavior", err)
		return false
	}
	ctx.Data["Diff"] = diff
	ctx.Data["DiffNotAvailable"] = diff.NumFiles == 0

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

	commits := git_model.ConvertFromGitCommit(ctx, ci.CompareInfo.Commits, ci.HeadRepo)
	ctx.Data["Commits"] = commits
	ctx.Data["CommitCount"] = len(commits)

	if len(commits) == 1 {
		c := commits[0]
		title = strings.TrimSpace(c.UserCommit.Summary())

		body := strings.Split(strings.TrimSpace(c.UserCommit.Message()), "\n")
		if len(body) > 1 {
			ctx.Data["content"] = strings.Join(body[1:], "\n")
		}
	} else {
		title = ci.HeadBranch
	}
	if len(title) > 255 {
		var trailer string
		title, trailer = util.SplitStringAtByteN(title, 255)
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

func getBranchesAndTagsForRepo(ctx gocontext.Context, repo *repo_model.Repository) (branches, tags []string, err error) {
	gitRepo, err := git.OpenRepository(ctx, repo.RepoPath())
	if err != nil {
		return nil, nil, err
	}
	defer gitRepo.Close()

	branches, err = git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID: repo.ID,
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		IsDeletedBranch: util.OptionalBoolFalse,
	})
	if err != nil {
		return nil, nil, err
	}
	tags, err = gitRepo.GetTags(0, 0)
	if err != nil {
		return nil, nil, err
	}
	return branches, tags, nil
}

// CompareDiff show different from one commit to another commit
func CompareDiff(ctx *context.Context) {
	ci := ParseCompareInfo(ctx)
	defer func() {
		if ci != nil && ci.HeadGitRepo != nil {
			ci.HeadGitRepo.Close()
		}
	}()
	if ctx.Written() {
		return
	}

	ctx.Data["PullRequestWorkInProgressPrefixes"] = setting.Repository.PullRequest.WorkInProgressPrefixes
	ctx.Data["DirectComparison"] = ci.DirectComparison
	ctx.Data["OtherCompareSeparator"] = ".."
	ctx.Data["CompareSeparator"] = "..."
	if ci.DirectComparison {
		ctx.Data["CompareSeparator"] = ".."
		ctx.Data["OtherCompareSeparator"] = "..."
	}

	nothingToCompare := PrepareCompareDiff(ctx, ci,
		gitdiff.GetWhitespaceFlag(ctx.Data["WhitespaceBehavior"].(string)))
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

	headBranches, err := git_model.FindBranchNames(ctx, git_model.FindBranchOptions{
		RepoID: ci.HeadRepo.ID,
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		IsDeletedBranch: util.OptionalBoolFalse,
	})
	if err != nil {
		ctx.ServerError("GetBranches", err)
		return
	}
	ctx.Data["HeadBranches"] = headBranches

	// For compare repo branches
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}

	headTags, err := repo_model.GetTagNamesByRepoID(ctx, ci.HeadRepo.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["HeadTags"] = headTags

	if ctx.Data["PageIsComparePull"] == true {
		pr, err := issues_model.GetUnmergedPullRequest(ctx, ci.HeadRepo.ID, ctx.Repo.Repository.ID, ci.HeadBranch, ci.BaseBranch, issues_model.PullRequestFlowGithub)
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
			RetrieveRepoMetas(ctx, ctx.Repo.Repository, true)
			if ctx.Written() {
				return
			}
		}
	}
	beforeCommitID := ctx.Data["BeforeCommitID"].(string)
	afterCommitID := ctx.Data["AfterCommitID"].(string)

	separator := "..."
	if ci.DirectComparison {
		separator = ".."
	}
	ctx.Data["Title"] = "Comparing " + base.ShortSha(beforeCommitID) + separator + base.ShortSha(afterCommitID)

	ctx.Data["IsRepoToolbarCommits"] = true
	ctx.Data["IsDiffCompare"] = true
	_, templateErrs := setTemplateIfExists(ctx, pullRequestTemplateKey, pullRequestTemplateCandidates)

	if len(templateErrs) > 0 {
		ctx.Flash.Warning(renderErrorOfTemplates(ctx, templateErrs), true)
	}

	if content, ok := ctx.Data["content"].(string); ok && content != "" {
		// If a template content is set, prepend the "content". In this case that's only
		// applicable if you have one commit to compare and that commit has a message.
		// In that case the commit message will be prepend to the template body.
		if templateContent, ok := ctx.Data[pullRequestTemplateKey].(string); ok && templateContent != "" {
			// Re-use the same key as that's priortized over the "content" key.
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
	commitID := ctx.Params("sha")
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
	if ctx.FormBool("wiki") {
		var err error
		gitRepo, err = git.OpenRepository(ctx, ctx.Repo.Repository.WikiPath())
		if err != nil {
			ctx.ServerError("OpenRepository", err)
			return
		}
		defer gitRepo.Close()
	}
	chunkSize := gitdiff.BlobExcerptChunkSize
	commit, err := gitRepo.GetCommit(commitID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit")
		return
	}
	section := &gitdiff.DiffSection{
		FileName: filePath,
		Name:     filePath,
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
		ctx.Error(http.StatusInternalServerError, "getExcerptLines")
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
		if direction == "up" {
			section.Lines = append([]*gitdiff.DiffLine{lineSection}, section.Lines...)
		} else if direction == "down" {
			section.Lines = append(section.Lines, lineSection)
		}
	}
	ctx.Data["section"] = section
	ctx.Data["FileNameHash"] = base.EncodeSha1(filePath)
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
	return diffLines, nil
}
