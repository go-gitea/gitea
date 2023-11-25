// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/upload"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/services/forms"
	releaseservice "code.gitea.io/gitea/services/release"
)

const (
	tplReleasesList base.TplName = "repo/release/list"
	tplReleaseNew   base.TplName = "repo/release/new"
	tplTagsList     base.TplName = "repo/tag/list"
)

// calReleaseNumCommitsBehind calculates given release has how many commits behind release target.
func calReleaseNumCommitsBehind(repoCtx *context.Repository, release *repo_model.Release, countCache map[string]int64) error {
	target := release.Target
	if target == "" {
		target = repoCtx.Repository.DefaultBranch
	}
	// Get count if not cached
	if _, ok := countCache[target]; !ok {
		commit, err := repoCtx.GitRepo.GetBranchCommit(target)
		if err != nil {
			var errNotExist git.ErrNotExist
			if target == repoCtx.Repository.DefaultBranch || !errors.As(err, &errNotExist) {
				return fmt.Errorf("GetBranchCommit: %w", err)
			}
			// fallback to default branch
			target = repoCtx.Repository.DefaultBranch
			commit, err = repoCtx.GitRepo.GetBranchCommit(target)
			if err != nil {
				return fmt.Errorf("GetBranchCommit(DefaultBranch): %w", err)
			}
		}
		countCache[target], err = commit.CommitsCount()
		if err != nil {
			return fmt.Errorf("CommitsCount: %w", err)
		}
	}
	release.NumCommitsBehind = countCache[target] - release.NumCommits
	release.TargetBehind = target
	return nil
}

// Releases render releases list page
func Releases(ctx *context.Context) {
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["Title"] = ctx.Tr("repo.release.releases")
	releasesOrTags(ctx, false)
}

// TagsList render tags list page
func TagsList(ctx *context.Context) {
	ctx.Data["PageIsTagList"] = true
	ctx.Data["Title"] = ctx.Tr("repo.release.tags")
	releasesOrTags(ctx, true)
}

func releasesOrTags(ctx *context.Context, isTagList bool) {
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch
	ctx.Data["IsViewBranch"] = false
	ctx.Data["IsViewTag"] = true
	// Disable the showCreateNewBranch form in the dropdown on this page.
	ctx.Data["CanCreateBranch"] = false
	ctx.Data["HideBranchesInDropdown"] = true

	listOptions := db.ListOptions{
		Page:     ctx.FormInt("page"),
		PageSize: ctx.FormInt("limit"),
	}
	if listOptions.PageSize == 0 {
		listOptions.PageSize = setting.Repository.Release.DefaultPagingNum
	}
	if listOptions.PageSize > setting.API.MaxResponseItems {
		listOptions.PageSize = setting.API.MaxResponseItems
	}

	// TODO(20073) tags are used for compare feature which needs all tags
	// filtering is done on the client-side atm
	tagListStart, tagListEnd := 0, 0
	if isTagList {
		tagListStart, tagListEnd = listOptions.GetStartEnd()
	}

	tags, err := ctx.Repo.GitRepo.GetTags(tagListStart, tagListEnd)
	if err != nil {
		ctx.ServerError("GetTags", err)
		return
	}
	ctx.Data["Tags"] = tags

	writeAccess := ctx.Repo.CanWrite(unit.TypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	opts := repo_model.FindReleasesOptions{
		ListOptions: listOptions,
	}
	if isTagList {
		// for the tags list page, show all releases with real tags (having real commit-id),
		// the drafts should also be included because a real tag might be used as a draft.
		opts.IncludeDrafts = true
		opts.IncludeTags = true
		opts.HasSha1 = util.OptionalBoolTrue
	} else {
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		opts.IncludeDrafts = writeAccess
	}

	releases, err := repo_model.GetReleasesByRepoID(ctx, ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	count, err := repo_model.GetReleaseCountByRepoID(ctx, ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.ServerError("GetReleaseCountByRepoID", err)
		return
	}

	for _, release := range releases {
		release.Repo = ctx.Repo.Repository
	}

	if err = repo_model.GetReleaseAttachments(ctx, releases...); err != nil {
		ctx.ServerError("GetReleaseAttachments", err)
		return
	}

	// Temporary cache commits count of used branches to speed up.
	countCache := make(map[string]int64)
	cacheUsers := make(map[int64]*user_model.User)
	if ctx.Doer != nil {
		cacheUsers[ctx.Doer.ID] = ctx.Doer
	}
	var ok bool

	for _, r := range releases {
		if r.Publisher, ok = cacheUsers[r.PublisherID]; !ok {
			r.Publisher, err = user_model.GetUserByID(ctx, r.PublisherID)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					r.Publisher = user_model.NewGhostUser()
				} else {
					ctx.ServerError("GetUserByID", err)
					return
				}
			}
			cacheUsers[r.PublisherID] = r.Publisher
		}

		r.Note, err = markdown.RenderString(&markup.RenderContext{
			URLPrefix: ctx.Repo.RepoLink,
			Metas:     ctx.Repo.Repository.ComposeMetas(),
			GitRepo:   ctx.Repo.GitRepo,
			Ctx:       ctx,
		}, r.Note)
		if err != nil {
			ctx.ServerError("RenderString", err)
			return
		}

		if r.IsDraft {
			continue
		}

		if err := calReleaseNumCommitsBehind(ctx.Repo, r, countCache); err != nil {
			ctx.ServerError("calReleaseNumCommitsBehind", err)
			return
		}
	}

	ctx.Data["Releases"] = releases

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	if isTagList {
		ctx.Data["PageIsViewCode"] = !ctx.Repo.Repository.UnitEnabled(ctx, unit.TypeReleases)
		ctx.HTML(http.StatusOK, tplTagsList)
	} else {
		ctx.HTML(http.StatusOK, tplReleasesList)
	}
}

// ReleasesFeedRSS get feeds for releases in RSS format
func ReleasesFeedRSS(ctx *context.Context) {
	releasesOrTagsFeed(ctx, true, "rss")
}

// TagsListFeedRSS get feeds for tags in RSS format
func TagsListFeedRSS(ctx *context.Context) {
	releasesOrTagsFeed(ctx, false, "rss")
}

// ReleasesFeedAtom get feeds for releases in Atom format
func ReleasesFeedAtom(ctx *context.Context) {
	releasesOrTagsFeed(ctx, true, "atom")
}

// TagsListFeedAtom get feeds for tags in RSS format
func TagsListFeedAtom(ctx *context.Context) {
	releasesOrTagsFeed(ctx, false, "atom")
}

func releasesOrTagsFeed(ctx *context.Context, isReleasesOnly bool, formatType string) {
	feed.ShowReleaseFeed(ctx, ctx.Repo.Repository, isReleasesOnly, formatType)
}

// SingleRelease renders a single release's page
func SingleRelease(ctx *context.Context) {
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch

	writeAccess := ctx.Repo.CanWrite(unit.TypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	release, err := repo_model.GetRelease(ctx.Repo.Repository.ID, ctx.Params("*"))
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetRelease", err)
			return
		}
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}
	ctx.Data["PageIsSingleTag"] = release.IsTag
	if release.IsTag {
		ctx.Data["Title"] = release.TagName
	} else {
		ctx.Data["Title"] = release.Title
	}

	release.Repo = ctx.Repo.Repository

	err = repo_model.GetReleaseAttachments(ctx, release)
	if err != nil {
		ctx.ServerError("GetReleaseAttachments", err)
		return
	}

	release.Publisher, err = user_model.GetUserByID(ctx, release.PublisherID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			release.Publisher = user_model.NewGhostUser()
		} else {
			ctx.ServerError("GetUserByID", err)
			return
		}
	}
	if !release.IsDraft {
		if err := calReleaseNumCommitsBehind(ctx.Repo, release, make(map[string]int64)); err != nil {
			ctx.ServerError("calReleaseNumCommitsBehind", err)
			return
		}
	}
	release.Note, err = markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Repo.RepoLink,
		Metas:     ctx.Repo.Repository.ComposeMetas(),
		GitRepo:   ctx.Repo.GitRepo,
		Ctx:       ctx,
	}, release.Note)
	if err != nil {
		ctx.ServerError("RenderString", err)
		return
	}

	ctx.Data["Releases"] = []*repo_model.Release{release}
	ctx.HTML(http.StatusOK, tplReleasesList)
}

// LatestRelease redirects to the latest release
func LatestRelease(ctx *context.Context) {
	release, err := repo_model.GetLatestReleaseByRepoID(ctx.Repo.Repository.ID)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound("LatestRelease", err)
			return
		}
		ctx.ServerError("GetLatestReleaseByRepoID", err)
		return
	}

	if err := release.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}

	ctx.Redirect(release.Link())
}

// NewRelease render creating or edit release page
func NewRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["tag_target"] = ctx.Repo.Repository.DefaultBranch
	if tagName := ctx.FormString("tag"); len(tagName) > 0 {
		rel, err := repo_model.GetRelease(ctx.Repo.Repository.ID, tagName)
		if err != nil && !repo_model.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		if rel != nil {
			rel.Repo = ctx.Repo.Repository
			if err := rel.LoadAttributes(ctx); err != nil {
				ctx.ServerError("LoadAttributes", err)
				return
			}

			ctx.Data["tag_name"] = rel.TagName
			if rel.Target != "" {
				ctx.Data["tag_target"] = rel.Target
			}
			ctx.Data["title"] = rel.Title
			ctx.Data["content"] = rel.Note
			ctx.Data["attachments"] = rel.Attachments
		}
	}
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = makeSelfOnTop(ctx, assigneeUsers)

	upload.AddUploadContext(ctx, "release")
	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// NewReleasePost response for creating a release
func NewReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplReleaseNew)
		return
	}

	if !ctx.Repo.GitRepo.IsBranchExist(form.Target) {
		ctx.RenderWithErr(ctx.Tr("form.target_branch_not_exist"), tplReleaseNew, &form)
		return
	}

	// Title of release cannot be empty
	if len(form.TagOnly) == 0 && len(form.Title) == 0 {
		ctx.RenderWithErr(ctx.Tr("repo.release.title_empty"), tplReleaseNew, &form)
		return
	}

	var attachmentUUIDs []string
	if setting.Attachment.Enabled {
		attachmentUUIDs = form.Files
	}

	rel, err := repo_model.GetRelease(ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !repo_model.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		msg := ""
		if len(form.Title) > 0 && form.AddTagMsg {
			msg = form.Title + "\n\n" + form.Content
		}

		if len(form.TagOnly) > 0 {
			if err = releaseservice.CreateNewTag(ctx, ctx.Doer, ctx.Repo.Repository, form.Target, form.TagName, msg); err != nil {
				if models.IsErrTagAlreadyExists(err) {
					e := err.(models.ErrTagAlreadyExists)
					ctx.Flash.Error(ctx.Tr("repo.branch.tag_collision", e.TagName))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				if models.IsErrInvalidTagName(err) {
					ctx.Flash.Error(ctx.Tr("repo.release.tag_name_invalid"))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				if models.IsErrProtectedTagName(err) {
					ctx.Flash.Error(ctx.Tr("repo.release.tag_name_protected"))
					ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
					return
				}

				ctx.ServerError("releaseservice.CreateNewTag", err)
				return
			}

			ctx.Flash.Success(ctx.Tr("repo.tag.create_success", form.TagName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/tag/" + util.PathEscapeSegments(form.TagName))
			return
		}

		rel = &repo_model.Release{
			RepoID:       ctx.Repo.Repository.ID,
			Repo:         ctx.Repo.Repository,
			PublisherID:  ctx.Doer.ID,
			Publisher:    ctx.Doer,
			Title:        form.Title,
			TagName:      form.TagName,
			Target:       form.Target,
			Note:         form.Content,
			IsDraft:      len(form.Draft) > 0,
			IsPrerelease: form.Prerelease,
			IsTag:        false,
		}

		if err = releaseservice.CreateRelease(ctx.Repo.GitRepo, rel, attachmentUUIDs, msg); err != nil {
			ctx.Data["Err_TagName"] = true
			switch {
			case repo_model.IsErrReleaseAlreadyExist(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
			case models.IsErrInvalidTagName(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_invalid"), tplReleaseNew, &form)
			case models.IsErrProtectedTagName(err):
				ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_protected"), tplReleaseNew, &form)
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
		rel.PublisherID = ctx.Doer.ID
		rel.IsTag = false

		if err = releaseservice.UpdateRelease(ctx.Doer, ctx.Repo.GitRepo, rel, attachmentUUIDs, nil, nil); err != nil {
			ctx.Data["Err_TagName"] = true
			ctx.ServerError("UpdateRelease", err)
			return
		}
	}
	log.Trace("Release created: %s/%s:%s", ctx.Doer.LowerName, ctx.Repo.Repository.Name, form.TagName)

	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// EditRelease render release edit page
func EditRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "release")

	tagName := ctx.Params("*")
	rel, err := repo_model.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
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

	rel.Repo = ctx.Repo.Repository
	if err := rel.LoadAttributes(ctx); err != nil {
		ctx.ServerError("LoadAttributes", err)
		return
	}
	ctx.Data["attachments"] = rel.Attachments

	// Get assignees.
	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, rel.Repo)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = makeSelfOnTop(ctx, assigneeUsers)

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// EditReleasePost response for edit release
func EditReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true

	tagName := ctx.Params("*")
	rel, err := repo_model.GetRelease(ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
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
		ctx.HTML(http.StatusOK, tplReleaseNew)
		return
	}

	const delPrefix = "attachment-del-"
	const editPrefix = "attachment-edit-"
	var addAttachmentUUIDs, delAttachmentUUIDs []string
	editAttachments := make(map[string]string) // uuid -> new name
	if setting.Attachment.Enabled {
		addAttachmentUUIDs = form.Files
		for k, v := range ctx.Req.Form {
			if strings.HasPrefix(k, delPrefix) && v[0] == "true" {
				delAttachmentUUIDs = append(delAttachmentUUIDs, k[len(delPrefix):])
			} else if strings.HasPrefix(k, editPrefix) {
				editAttachments[k[len(editPrefix):]] = v[0]
			}
		}
	}

	rel.Title = form.Title
	rel.Note = form.Content
	rel.IsDraft = len(form.Draft) > 0
	rel.IsPrerelease = form.Prerelease
	if err = releaseservice.UpdateRelease(ctx.Doer, ctx.Repo.GitRepo,
		rel, addAttachmentUUIDs, delAttachmentUUIDs, editAttachments); err != nil {
		ctx.ServerError("UpdateRelease", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// DeleteRelease deletes a release
func DeleteRelease(ctx *context.Context) {
	deleteReleaseOrTag(ctx, false)
}

// DeleteTag deletes a tag
func DeleteTag(ctx *context.Context) {
	deleteReleaseOrTag(ctx, true)
}

func deleteReleaseOrTag(ctx *context.Context, isDelTag bool) {
	redirect := func() {
		if isDelTag {
			ctx.JSON(http.StatusOK, map[string]any{
				"redirect": ctx.Repo.RepoLink + "/tags",
			})
			return
		}

		ctx.JSON(http.StatusOK, map[string]any{
			"redirect": ctx.Repo.RepoLink + "/releases",
		})
	}

	rel, err := repo_model.GetReleaseForRepoByID(ctx, ctx.Repo.Repository.ID, ctx.FormInt64("id"))
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound("GetReleaseForRepoByID", err)
		} else {
			ctx.Flash.Error("DeleteReleaseByID: " + err.Error())
			redirect()
		}
		return
	}

	if err := releaseservice.DeleteReleaseByID(ctx, ctx.Repo.Repository, rel, ctx.Doer, isDelTag); err != nil {
		if models.IsErrProtectedTagName(err) {
			ctx.Flash.Error(ctx.Tr("repo.release.tag_name_protected"))
		} else {
			ctx.Flash.Error("DeleteReleaseByID: " + err.Error())
		}
	} else {
		if isDelTag {
			ctx.Flash.Success(ctx.Tr("repo.release.deletion_tag_success"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.release.deletion_success"))
		}
	}

	redirect()
}
