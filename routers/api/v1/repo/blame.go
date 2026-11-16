// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/services/context"
	"gitea.dev/services/convert"
)

// GetBlame returns the blame of a file
func GetBlame(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/blame/{filepath} repository repoGetBlame
	// ---
	// summary: Get the blame of a file, the commit that last changed each of its lines
	// description: Line contents are not included, get them from `/repos/{owner}/{repo}/raw/{filepath}?ref={commit_id}`. Commits listed in the file's `.git-blame-ignore-revs` are skipped, as in the web UI, unless `bypass_ignore_revs` is set.
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
	// - name: filepath
	//   in: path
	//   description: path of the file
	//   type: string
	//   required: true
	// - name: ref
	//   in: query
	//   description: the name of the commit/branch/tag, default to the repository’s default branch.
	//   type: string
	//   required: false
	// - name: bypass_ignore_revs
	//   in: query
	//   description: don't skip the commits listed in `.git-blame-ignore-revs`
	//   type: boolean
	//   required: false
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileBlame"
	//   default:
	//     "$ref": "#/responses/error"

	refCommit := resolveRefCommit(ctx, ctx.FormTrim("ref"))
	if ctx.Written() {
		return
	}
	treePath := ctx.PathParam("*")
	blob, err := refCommit.Commit.GetBlobByPath(ctx, ctx.Repo.GitRepo, treePath)
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}
	if blob.Size(ctx) >= setting.UI.MaxDisplayFileSize {
		ctx.APIError(http.StatusUnprocessableEntity, "file is too large to blame")
		return
	}

	result, err := git.GetBlame(ctx, ctx.Repo.GitRepo, refCommit.Commit, treePath, ctx.FormBool("bypass_ignore_revs"))
	if err != nil {
		ctx.APIErrorAuto(err)
		return
	}

	commits := make(map[string]*api.BlameCommit)
	ranges := make([]*api.BlameRange, 0, len(result.Parts))
	line := 1
	for _, part := range result.Parts {
		commit, ok := commits[part.Sha]
		if !ok {
			gitCommit, err := ctx.Repo.GitRepo.GetCommit(ctx, part.Sha)
			if err != nil {
				ctx.APIErrorAuto(err)
				return
			}
			commit = &api.BlameCommit{
				SHA:       part.Sha,
				URL:       ctx.Repo.Repository.APIURL(ctx) + "/git/commits/" + part.Sha,
				Author:    convert.ToCommitUser(gitCommit.Author),
				Committer: convert.ToCommitUser(gitCommit.Committer),
				Summary:   gitCommit.MessageTitle(),
			}
			commits[part.Sha] = commit
		}
		ranges = append(ranges, &api.BlameRange{
			StartLine: line,
			EndLine:   line + len(part.Lines) - 1,
			Commit:    commit,
		})
		line += len(part.Lines)
	}

	ctx.JSON(http.StatusOK, &api.FileBlame{
		CommitID:       refCommit.CommitID,
		Path:           treePath,
		UsesIgnoreRevs: result.UsesIgnoreRevs,
		Ranges:         ranges,
	})
}
