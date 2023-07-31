// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/gitdiff"
	repo_service "code.gitea.io/gitea/services/repository"
)

func getWhitespaceBehavior(ctx *context.APIContext) string {
	const defaultWhitespaceBehavior = "show-all"

	whitespaceBehavior := ctx.FormString("whitespace")
	switch whitespaceBehavior {
	case "", "ignore-all", "ignore-eol", "ignore-change":
		break
	default:
		whitespaceBehavior = defaultWhitespaceBehavior
	}

	if ctx.IsSigned && whitespaceBehavior == "" {
		userWhitespaceBehavior, err := user_model.GetUserSetting(ctx.Doer.ID, user_model.SettingsKeyDiffWhitespaceBehavior, defaultWhitespaceBehavior)
		if err == nil {
			return userWhitespaceBehavior
		}
	}

	// these behaviors are for gitdiff.GetWhitespaceFlag
	if whitespaceBehavior == "" {
		return defaultWhitespaceBehavior
	}
	return whitespaceBehavior
}

func Compare(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/compare/{diffpath} repository repoCompare
	// ---
	// summary: compare commits
	// produces:
	// - application/json
	// parameters:
	//   - name: owner
	//     in: path
	//     description: owner of the base repo
	//     type: string
	//     required: true
	//   - name: repo
	//     in: path
	//     description: name of the base repo
	//     type: string
	//     required: true
	//   - name: diffpath
	//     in: path
	//     description: compare info of refs
	//     type: string
	//     required: true
	//   - name: skip-patch
	//     in: query
	//     description: do not response diff patch
	//     type: boolean
	// responses:
	//   "200":
	//     "$ref": "#/responses/GitCompareResponse"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ci, err := repo_service.ParseCompareInfo(ctx.Base, ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.Doer)
	if err != nil {
		if repo_service.IsErrCompareNotFound(err) {
			ctx.NotFound(err)
			return
		}

		ctx.ServerError("ParseCompareInfo", err)
		return
	}
	defer func() {
		if ci != nil && ci.HeadGitRepo != nil {
			ci.HeadGitRepo.Close()
		}
	}()
	if ctx.Written() {
		return
	}

	diff, _, err := ci.LoadCompareDiff(gitdiff.GetWhitespaceFlag(getWhitespaceBehavior(ctx)),
		nil, "")
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadCompareDiff", err)
		return
	}

	result := &structs.GitCompareResponse{
		URL:     setting.AppURL + "api/v1/repos/" + ctx.Repo.Repository.OwnerName + "/" + ctx.Repo.Repository.Name + "/compare/" + ctx.Params("*"),
		HTMLURL: ctx.Repo.Repository.HTMLURL() + "/compare/" + ctx.Params("*"),
	}

	userCache := make(map[string]*user_model.User)

	if ci.DirectComparison {
		result.BaseCommit, err = convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ci.BeforeCommit, userCache, convert.ToCommitOptions{})
	} else {
		result.MergeBaseCommit, err = convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, ci.BeforeCommit, userCache, convert.ToCommitOptions{})
	}
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "ToCommit", err)
		return
	}

	if len(ci.CompareInfo.Commits) > 0 {
		result.Commits = make([]*structs.Commit, 0, len(ci.CompareInfo.Commits))
		for _, commit := range ci.CompareInfo.Commits {
			exist := ci.HeadGitRepo != nil && ci.HeadGitRepo.IsCommitExist(commit.ID.String())

			var apiCommit *structs.Commit

			if exist {
				apiCommit, err = convert.ToCommit(ctx, ci.HeadRepo, ci.HeadGitRepo, commit, userCache, convert.ToCommitOptions{})
			} else {
				apiCommit, err = convert.ToCommit(ctx, ctx.Repo.Repository, ctx.Repo.GitRepo, commit, userCache, convert.ToCommitOptions{})
			}

			if err != nil {
				ctx.Error(http.StatusInternalServerError, "ToCommit", err)
				return
			}

			result.Commits = append(result.Commits, apiCommit)
		}
	}

	result.TotalCommits = int64(len(result.Commits))
	// TODO:
	// result.AheadBy = ?
	// result.BehindBy = ?

	skipPatch := ctx.FormBool("skip-patch")

	if diff != nil && len(diff.Files) > 0 {
		result.Files = make([]*structs.GitCompareFile, 0, len(diff.Files))

		for _, diffFile := range diff.Files {
			apiDiffFile := &structs.GitCompareFile{
				FileName:    diffFile.Name,
				OldFileName: diffFile.OldName,
				SHA:         diffFile.NameHash,
				Additions:   int64(diffFile.Addition),
				Deletions:   int64(diffFile.Deletion),
				Changes:     int64(diffFile.Addition + diffFile.Deletion),
			}

			if diffFile.Type == gitdiff.DiffFileAdd {
				apiDiffFile.Status = "added"
			} else if diffFile.Type == gitdiff.DiffFileDel {
				apiDiffFile.Status = "deleted"
			} else if diffFile.Type == gitdiff.DiffFileRename {
				apiDiffFile.Status = "renamed"
			} else if diffFile.Type == gitdiff.DiffFileChange {
				apiDiffFile.Status = "modified"
			} else if diffFile.Type == gitdiff.DiffFileCopy {
				apiDiffFile.Status = "copied"
			} else {
				apiDiffFile.Status = "unknow"
			}

			if skipPatch {
				result.Files = append(result.Files, apiDiffFile)
				continue
			}

			var patchBuilder bytes.Buffer
			for _, diffSec := range diffFile.Sections {
				for _, diffLine := range diffSec.Lines {
					patchBuilder.WriteString(diffLine.Content)
					patchBuilder.WriteString("\n")
				}
			}
			apiDiffFile.Patch = patchBuilder.String()

			result.Files = append(result.Files, apiDiffFile)
		}
	}

	ctx.JSON(http.StatusOK, result)
}
