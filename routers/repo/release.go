// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	releaseservice "code.gitea.io/gitea/services/release"
)

const (
	tplReleases   base.TplName = "repo/release/list"
	tplReleaseNew base.TplName = "repo/release/new"
)

// calReleaseNumCommitsBehind calculates given release has how many commits behind release target.
func calReleaseNumCommitsBehind(repoCtx *context.Repository, release *models.Release, countCache map[string]int64) error {
	// Fast return if release target is same as default branch.
	if repoCtx.BranchName == release.Target {
		release.NumCommitsBehind = repoCtx.CommitsCount - release.NumCommits
		return nil
	}

	// Get count if not exists
	if _, ok := countCache[release.Target]; !ok {
		if repoCtx.GitRepo.IsBranchExist(release.Target) {
			commit, err := repoCtx.GitRepo.GetBranchCommit(release.Target)
			if err != nil {
				return fmt.Errorf("GetBranchCommit: %v", err)
			}
			countCache[release.Target], err = commit.CommitsCount()
			if err != nil {
				return fmt.Errorf("CommitsCount: %v", err)
			}
		} else {
			// Use NumCommits of the newest release on that target
			countCache[release.Target] = release.NumCommits
		}
	}
	release.NumCommitsBehind = countCache[release.Target] - release.NumCommits
	return nil
}

// Releases render releases list page
func Releases(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.releases")
	ctx.Data["PageIsReleaseList"] = true

	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}
	limit := ctx.QueryInt("limit")
	if limit <= 0 {
		limit = 10
	}

	writeAccess := ctx.Repo.CanWrite(models.UnitTypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	opts := models.FindReleasesOptions{
		IncludeDrafts: writeAccess,
		IncludeTags:   true,
	}

	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, opts, page, limit)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	count, err := models.GetReleaseCountByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}

	err = models.GetReleaseAttachments(releases...)
	if err != nil {
		ctx.ServerError("GetReleaseAttachments", err)
		return
	}

	// Temporary cache commits count of used branches to speed up.
	countCache := make(map[string]int64)
	cacheUsers := make(map[int64]*models.User)
	if ctx.User != nil {
		cacheUsers[ctx.User.ID] = ctx.User
	}
	var ok bool

	for _, r := range releases {
		if r.Publisher, ok = cacheUsers[r.PublisherID]; !ok {
			r.Publisher, err = models.GetUserByID(r.PublisherID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					r.Publisher = models.NewGhostUser()
				} else {
					ctx.ServerError("GetUserByID", err)
					return
				}
			}
			cacheUsers[r.PublisherID] = r.Publisher
		}
		if err := calReleaseNumCommitsBehind(ctx.Repo, r, countCache); err != nil {
			ctx.ServerError("calReleaseNumCommitsBehind", err)
			return
		}
		r.Note = markdown.RenderString(r.Note, ctx.Repo.RepoLink, ctx.Repo.Repository.ComposeMetas())
	}

	ctx.Data["Releases"] = releases

	pager := context.NewPagination(int(count), limit, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(200, tplReleases)
}

// NewRelease render creating release page
func NewRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["tag_target"] = ctx.Repo.Repository.DefaultBranch
	renderAttachmentSettings(ctx)
	ctx.HTML(200, tplReleaseNew)
}

// NewReleasePost response for creating a release
func NewReleasePost(ctx *context.Context, form auth.NewReleaseForm) {
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true

	if ctx.HasError() {
		ctx.HTML(200, tplReleaseNew)
		return
	}

	if !ctx.Repo.GitRepo.IsBranchExist(form.Target) {
		ctx.RenderWithErr(ctx.Tr("form.target_branch_not_exist"), tplReleaseNew, &form)
		return
	}

	var attachmentUUIDs []string
	if setting.AttachmentEnabled {
		attachmentUUIDs = form.Files
	}

	rel, err := models.GetRelease(ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !models.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		rel := &models.Release{
			RepoID:       ctx.Repo.Repository.ID,
			PublisherID:  ctx.User.ID,
			Title:        form.Title,
			TagName:      form.TagName,
			Target:       form.Target,
			Note:         form.Content,
			IsDraft:      len(form.Draft) > 0,
			IsPrerelease: form.Prerelease,
			IsTag:        false,
		}

		if err = releaseservice.CreateRelease(ctx.Repo.GitRepo, rel, attachmentUUIDs); err != nil {
			ctx.Data["Err_TagName"] = true
			switch {
			case models.IsErrReleaseAlreadyExist(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
			case models.IsErrInvalidTagName(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_invalid"), tplReleaseNew, &form)
			default:
				ctx.ServerError("CreateRelease", err)
			}
			return
		}
	} else {
		if !rel.IsTag {
			ctx.Data["Err_TagName"] = true
			ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
			return
		}

		rel.Title = form.Title
		rel.Note = form.Content
		rel.Target = form.Target
		rel.IsDraft = len(form.Draft) > 0
		rel.IsPrerelease = form.Prerelease
		rel.PublisherID = ctx.User.ID
		rel.IsTag = false

		if err = releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo, rel, attachmentUUIDs); err != nil {
			ctx.Data["Err_TagName"] = true
			ctx.ServerError("UpdateRelease", err)
			return
		}
	}
	log.Trace("Release created: %s/%s:%s", ctx.User.LowerName, ctx.Repo.Repository.Name, form.TagName)

	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// EditRelease render release edit page
func EditRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true
	renderAttachmentSettings(ctx)

	tagName := ctx.Params("*")
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
		} else {
			ctx.ServerError("GetRelease", err)
		}
		return
	}
	ctx.Data["ID"] = rel.ID
	ctx.Data["tag_name"] = rel.TagName
	ctx.Data["tag_target"] = rel.Target
	ctx.Data["title"] = rel.Title
	ctx.Data["content"] = rel.Note
	ctx.Data["prerelease"] = rel.IsPrerelease
	ctx.Data["IsDraft"] = rel.IsDraft

	ctx.HTML(200, tplReleaseNew)
}

// EditReleasePost response for edit release
func EditReleasePost(ctx *context.Context, form auth.EditReleaseForm) {
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true

	tagName := ctx.Params("*")
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if models.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
		} else {
			ctx.ServerError("GetRelease", err)
		}
		return
	}
	if rel.IsTag {
		ctx.NotFound("GetRelease", err)
		return
	}
	ctx.Data["tag_name"] = rel.TagName
	ctx.Data["tag_target"] = rel.Target
	ctx.Data["title"] = rel.Title
	ctx.Data["content"] = rel.Note
	ctx.Data["prerelease"] = rel.IsPrerelease

	if ctx.HasError() {
		ctx.HTML(200, tplReleaseNew)
		return
	}

	var attachmentUUIDs []string
	if setting.AttachmentEnabled {
		attachmentUUIDs = form.Files
	}

	rel.Title = form.Title
	rel.Note = form.Content
	rel.IsDraft = len(form.Draft) > 0
	rel.IsPrerelease = form.Prerelease
	if err = releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo, rel, attachmentUUIDs); err != nil {
		ctx.ServerError("UpdateRelease", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// DeleteRelease delete a release
func DeleteRelease(ctx *context.Context) {
	if err := releaseservice.DeleteReleaseByID(ctx.QueryInt64("id"), ctx.User, true); err != nil {
		ctx.Flash.Error("DeleteReleaseByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.release.deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/releases",
	})
}
