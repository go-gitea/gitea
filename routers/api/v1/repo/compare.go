// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/gitdiff"
)

// GetCompare get a comparison of the two versions via shaes / branches / tags
func GetCompare(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/git/compare/{sha} repository repoGetCompare
	// ---
	// summary: Get a comparison of the two versions via shaes / branches / tags
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
	// - name: sha
	//   in: path
	//   description: The version to compare, use '...' to split
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Compare"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, baseIs, headIs, hasErr := parseCompareDiffInfo(ctx)
	if hasErr {
		return
	}
	defer headGitRepo.Close()

	getCompareDiff(ctx, headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, baseIs, headIs)
}

func getCompareDiff(ctx *context.APIContext, headUser *models.User, headRepo *models.Repository, headGitRepo *git.Repository, compareInfo *git.CompareInfo, baseBranch string, headBranch string, baseIs, headIs map[string]bool) {
	if err := parseBaseRepoInfo(ctx, headRepo); err != nil {
		ctx.Error(http.StatusInternalServerError, "parseBaseRepoInfo", err)
		return
	}

	headCommitID, baseCommitID, diff, commitCount, headTarget, nothingToCompare, hasErr := prepareCompareDiff(ctx, headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, baseIs, headIs)
	if hasErr {
		return
	}

	compare := new(api.Compare)
	if nothingToCompare {
		compare.Title = "Comparing " + baseBranch + "..." + headBranch + "is the same commit"
	} else {
		compare.Title = "Comparing " + base.ShortSha(baseCommitID) + "..." + base.ShortSha(headCommitID)
	}

	err := toCompare(ctx, compare, compareInfo, headCommitID, baseCommitID, diff, commitCount, headTarget)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "toCompare", err)
		return
	}
	ctx.JSON(http.StatusOK, compare)
}

func parseBaseRepoInfo(ctx *context.APIContext, headRepo *models.Repository) error {
	if !headRepo.IsFork {
		return nil
	}
	return headRepo.GetBaseRepo()
}

func toCompare(ctx *context.APIContext, compare *api.Compare, compareInfo *git.CompareInfo, headCommitID, baseCommitID string, diff *gitdiff.Diff, commitCount int, headTarget string) error {
	// Generate the BaseCommitID and HeadCommitID in the compare.
	compare.BaseCommitID, compare.HeadCommitID = baseCommitID, headCommitID

	// Generate the Commits and CommitCount in the compare.
	if err := toCommits(ctx, compare, compareInfo, commitCount); err != nil {
		return err
	}

	// Generate the Diff in the compare.
	return toDiff(ctx, compare, diff)
}

func parseCompareDiffInfo(ctx *context.APIContext) (*models.User, *models.Repository, *git.Repository, *git.CompareInfo, string, string, map[string]bool, map[string]bool, bool) {
	baseRepo := ctx.Repo.Repository

	// Get compared branches | shaes | tags information
	// format: <base branch>...[<head repo>:]<head branch>
	// base<-head: master...head:feature
	// same repo: master...feature

	headUser, baseBranch, headBranch, isSameRepo, hasErr := parseParams(ctx, baseRepo)
	if hasErr {
		return nil, nil, nil, nil, "", "", nil, nil, true
	}

	// Check if base branch is valid.
	baseIs := make(map[string]bool, 3)
	baseBranch, baseIs["commit"], baseIs["branch"], baseIs["tag"], hasErr = checkBranch(ctx, ctx.Repo.GitRepo, baseBranch)
	if hasErr {
		return nil, nil, nil, nil, "", "", nil, nil, true
	}

	// Check if current user has fork of repository or in the same repository.
	headRepo, _ := models.HasForkedRepo(headUser.ID, baseRepo.ID)

	var headGitRepo *git.Repository
	var err error
	if isSameRepo {
		headRepo = ctx.Repo.Repository
		headGitRepo = ctx.Repo.GitRepo
	} else {
		headGitRepo, err = git.OpenRepository(models.RepoPath(headUser.Name, headRepo.Name))
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "OpenRepository", err)
			return nil, nil, nil, nil, "", "", nil, nil, true
		}
		defer headGitRepo.Close()
	}

	// user should have permission to read baseRepo's codes and pulls, NOT headRepo's
	permBase, err := models.GetUserRepoPermission(baseRepo, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return nil, nil, nil, nil, "", "", nil, nil, true
	}
	if !permBase.CanRead(models.UnitTypeCode) {
		if log.IsTrace() {
			log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in baseRepo has Permissions: %-+v",
				ctx.User,
				baseRepo,
				permBase)
		}
		ctx.NotFound("ParseCompareDiffInfo", nil)
		return nil, nil, nil, nil, "", "", nil, nil, true
	}

	if !isSameRepo {
		// user should have permission to read headrepo's codes
		permHead, err := models.GetUserRepoPermission(headRepo, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return nil, nil, nil, nil, "", "", nil, nil, true
		}
		if !permHead.CanRead(models.UnitTypeCode) {
			if log.IsTrace() {
				log.Trace("Permission Denied: User: %-v cannot read code in Repo: %-v\nUser in headRepo has Permissions: %-+v",
					ctx.User,
					headRepo,
					permHead)
			}
			ctx.NotFound("ParseCompareDiffInfo", nil)
			return nil, nil, nil, nil, "", "", nil, nil, true
		}
	}

	// Check if head branch is valid.
	headIs := make(map[string]bool, 3)
	headBranch, headIs["commit"], headIs["branch"], headIs["tag"], hasErr = checkBranch(ctx, headGitRepo, headBranch)
	if hasErr {
		return nil, nil, nil, nil, "", "", nil, nil, true
	}

	compareInfo, err := headGitRepo.GetCompareInfo(baseRepo.RepoPath(), baseBranch, headBranch)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCompareInfo", err)
		return nil, nil, nil, nil, "", "", nil, nil, true
	}

	return headUser, headRepo, headGitRepo, compareInfo, baseBranch, headBranch, baseIs, headIs, false
}

func parseParams(ctx *context.APIContext, baseRepo *models.Repository) (*models.User, string, string, bool, bool) {
	infoPath := ctx.Params("*")
	infos := strings.Split(infoPath, "...")
	if len(infos) != 2 {
		log.Trace("ParseCompareDiffInfo[%d]: not enough compared branches information %s", baseRepo.ID, infos)
		ctx.NotFound("Compare", nil)
		return nil, "", "", false, true
	}

	baseBranch := infos[0]

	var (
		headUser   *models.User
		isSameRepo bool
		headBranch string
		err        error
	)

	// If there is no head repository, it means compare between same repository.
	headInfos := strings.Split(infos[1], ":")
	if len(headInfos) == 1 {
		isSameRepo = true
		headUser = ctx.Repo.Owner
		headBranch = headInfos[0]
	} else if len(headInfos) == 2 {
		headUser, err = models.GetUserByName(headInfos[0])
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("GetUserByName", nil)
			} else {
				ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
			}
			return nil, "", "", false, true
		}
		headBranch = headInfos[1]
		isSameRepo = headUser.ID == ctx.Repo.Owner.ID
	} else {
		ctx.NotFound("Compare", nil)
		return nil, "", "", false, true
	}

	ctx.Context.Repo.PullRequest.SameRepo = isSameRepo

	return headUser, baseBranch, headBranch, isSameRepo, false
}

func checkBranch(ctx *context.APIContext, gitRepo *git.Repository, branch string) (string, bool, bool, bool, bool) {
	isCommit := gitRepo.IsCommitExist(branch)
	isBranch := gitRepo.IsBranchExist(branch)
	isTag := gitRepo.IsTagExist(branch)
	if !isCommit && !isBranch && !isTag {
		// Check if baseBranch is short sha commit hash
		if commit, _ := gitRepo.GetCommit(branch); commit != nil {
			branch = commit.ID.String()
			isCommit = true
		} else {
			ctx.NotFound("IsRefExist", nil)
			return "", false, false, false, true
		}
	}
	return branch, isCommit, isBranch, isTag, false
}

func prepareCompareDiff(
	ctx *context.APIContext,
	headUser *models.User,
	headRepo *models.Repository,
	headGitRepo *git.Repository,
	compareInfo *git.CompareInfo,
	baseBranch, headBranch string,
	baseIs, headIs map[string]bool) (string, string, *gitdiff.Diff, int, string, bool, bool) {

	var (
		repo = ctx.Repo.Repository
		err  error

		// return
		headCommitID string
		baseCommitID string
		diff         *gitdiff.Diff
		commitCount  int
		headTarget   string
	)

	// Get diff information.
	headCommitID = headBranch
	if !headIs["commit"] {
		if headIs["tag"] {
			headCommitID, err = headGitRepo.GetTagCommitID(headBranch)
		} else {
			headCommitID, err = headGitRepo.GetBranchCommitID(headBranch)
		}
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetRefCommitID", err)
			return "", "", nil, 0, "", false, true
		}
	}

	headTarget = path.Join(headUser.Name, repo.Name)

	if headCommitID == compareInfo.MergeBase {
		return headCommitID, compareInfo.MergeBase, nil, 0, headTarget, true, false
	}

	diff, err = gitdiff.GetDiffRange(models.RepoPath(headUser.Name, headRepo.Name),
		compareInfo.MergeBase, headCommitID, setting.Git.MaxGitDiffLines,
		setting.Git.MaxGitDiffLineCharacters, setting.Git.MaxGitDiffFiles)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetDiffRange", err)
		return "", "", nil, 0, "", false, true
	}

	baseGitRepo := ctx.Repo.GitRepo
	baseCommitID = baseBranch
	if !baseIs["commit"] {
		if baseIs["tag"] {
			baseCommitID, err = baseGitRepo.GetTagCommitID(baseBranch)
		} else {
			baseCommitID, err = baseGitRepo.GetBranchCommitID(baseBranch)
		}
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetRefCommitID", err)
			return "", "", nil, 0, "", false, true
		}
	}

	commitCount = compareInfo.Commits.Len()

	return headCommitID, baseCommitID, diff, commitCount, headTarget, false, false
}

func toCommits(ctx *context.APIContext, compare *api.Compare, compareInfo *git.CompareInfo, commitCount int) error {
	compare.Commits = make([]*api.Commit, 0, commitCount)
	compare.CommitCount = commitCount

	// From list to array.
	for cur := compareInfo.Commits.Front(); cur != nil; cur = cur.Next() {
		// Convert git.Commit to api.Commit.
		commit, err := toCommit(ctx, ctx.Repo.Repository, cur.Value.(*git.Commit), nil)
		if err != nil {
			return err
		}
		compare.Commits = append(compare.Commits, commit)
	}
	return nil
}

func toDiff(ctx *context.APIContext, compare *api.Compare, diff *gitdiff.Diff) error {
	compare.Diff = new(api.Diff)
	compare.Diff.TotalAddition = diff.TotalAddition
	compare.Diff.TotalDeletion = diff.TotalDeletion
	for i := range diff.Files {
		compareDiffFile, err := toCompareDiff(ctx, diff.Files[i])
		if err != nil {
			return err
		}
		compare.Diff.Files = append(compare.Diff.Files, compareDiffFile)
	}
	compare.Diff.IsIncomplete = diff.IsIncomplete
	return nil
}

func toCompareDiff(ctx *context.APIContext, compareDiffFile *gitdiff.DiffFile) (*api.DiffFile, error) {
	apiDiffFile := new(api.DiffFile)
	apiDiffFile.Name = compareDiffFile.Name
	apiDiffFile.OldName = compareDiffFile.OldName
	apiDiffFile.Index = compareDiffFile.Index
	apiDiffFile.Addition = compareDiffFile.Addition
	apiDiffFile.Deletion = compareDiffFile.Deletion
	apiDiffFile.Type = int(compareDiffFile.Type)
	apiDiffFile.IsCreated = compareDiffFile.IsCreated
	apiDiffFile.IsDeleted = compareDiffFile.IsDeleted
	apiDiffFile.IsBin = compareDiffFile.IsBin
	apiDiffFile.IsLFSFile = compareDiffFile.IsLFSFile
	apiDiffFile.IsRenamed = compareDiffFile.IsRenamed
	apiDiffFile.IsSubmodule = compareDiffFile.IsSubmodule
	for i := range compareDiffFile.Sections {
		apiDiffSection, err := toCompareDiffSection(ctx, compareDiffFile.Sections[i])
		if err != nil {
			return nil, err
		}
		apiDiffFile.Sections = append(apiDiffFile.Sections, apiDiffSection)
	}
	apiDiffFile.IsIncomplete = compareDiffFile.IsIncomplete
	return apiDiffFile, nil
}

func toCompareDiffSection(ctx *context.APIContext, compareDiffFileSection *gitdiff.DiffSection) (*api.DiffSection, error) {
	apiDiffSection := new(api.DiffSection)
	apiDiffSection.Name = compareDiffFileSection.Name
	for i := range compareDiffFileSection.Lines {
		apiDiffSectionLine, err := toCompareDiffSectionDiffLine(ctx, compareDiffFileSection.Lines[i])
		if err != nil {
			return nil, err
		}
		apiDiffSection.Lines = append(apiDiffSection.Lines, apiDiffSectionLine)
	}
	return apiDiffSection, nil
}

func toCompareDiffSectionDiffLine(ctx *context.APIContext, apiDiffSectionLine *gitdiff.DiffLine) (*api.DiffLine, error) {
	compareDiffFileSectionLine := new(api.DiffLine)
	compareDiffFileSectionLine.LeftIdx = apiDiffSectionLine.LeftIdx
	compareDiffFileSectionLine.RightIdx = apiDiffSectionLine.RightIdx
	compareDiffFileSectionLine.Type = int(apiDiffSectionLine.Type)
	compareDiffFileSectionLine.Content = apiDiffSectionLine.Content
	for i := range apiDiffSectionLine.Comments {
		comment, err := toComment(ctx, apiDiffSectionLine.Comments[i])
		if err != nil {
			return nil, err
		}
		compareDiffFileSectionLine.Comments = append(compareDiffFileSectionLine.Comments, comment)
	}
	if apiDiffSectionLine.SectionInfo == nil {
		return compareDiffFileSectionLine, nil
	}
	compareDiffFileSectionLine.SectionInfo = &api.DiffLineSectionInfo{
		Path:          apiDiffSectionLine.SectionInfo.Path,
		LastLeftIdx:   apiDiffSectionLine.SectionInfo.LastLeftIdx,
		LastRightIdx:  apiDiffSectionLine.SectionInfo.LastRightIdx,
		LeftIdx:       apiDiffSectionLine.SectionInfo.LeftIdx,
		RightIdx:      apiDiffSectionLine.SectionInfo.RightIdx,
		LeftHunkSize:  apiDiffSectionLine.SectionInfo.LeftHunkSize,
		RightHunkSize: apiDiffSectionLine.SectionInfo.RightHunkSize,
	}
	return compareDiffFileSectionLine, nil
}

func toComment(ctx *context.APIContext, comment *models.Comment) (*api.Comment, error) {
	var err error
	if err = comment.LoadIssue(); err != nil {
		return nil, err
	}
	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		return nil, err
	}

	if comment.Type != models.CommentTypeComment {
		return nil, err
	}

	if err = comment.LoadPoster(); err != nil {
		return nil, err
	}

	return comment.APIFormat(), nil
}
