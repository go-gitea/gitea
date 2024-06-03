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
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/feed"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
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

type ReleaseInfo struct {
	Release        *repo_model.Release
	CommitStatus   *git_model.CommitStatus
	CommitStatuses []*git_model.CommitStatus
}

func getReleaseInfos(ctx *context.Context, opts *repo_model.FindReleasesOptions) ([]*ReleaseInfo, error) {
	releases, err := db.Find[repo_model.Release](ctx, opts)
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		release.Repo = ctx.Repo.Repository
	}

	if err = repo_model.GetReleaseAttachments(ctx, releases...); err != nil {
		return nil, err
	}

	// Temporary cache commits count of used branches to speed up.
	countCache := make(map[string]int64)
	cacheUsers := make(map[int64]*user_model.User)
	if ctx.Doer != nil {
		cacheUsers[ctx.Doer.ID] = ctx.Doer
	}
	var ok bool

	canReadActions := ctx.Repo.CanRead(unit.TypeActions)

	releaseInfos := make([]*ReleaseInfo, 0, len(releases))
	for _, r := range releases {
		if r.Publisher, ok = cacheUsers[r.PublisherID]; !ok {
			r.Publisher, err = user_model.GetUserByID(ctx, r.PublisherID)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					r.Publisher = user_model.NewGhostUser()
				} else {
					return nil, err
				}
			}
			cacheUsers[r.PublisherID] = r.Publisher
		}

		r.RenderedNote, err = markdown.RenderString(&markup.RenderContext{
			Links: markup.Links{
				Base: ctx.Repo.RepoLink,
			},
			Metas:   ctx.Repo.Repository.ComposeMetas(ctx),
			GitRepo: ctx.Repo.GitRepo,
			Repo:    ctx.Repo.Repository,
			Ctx:     ctx,
		}, r.Note)
		if err != nil {
			return nil, err
		}

		if !r.IsDraft {
			if err := calReleaseNumCommitsBehind(ctx.Repo, r, countCache); err != nil {
				return nil, err
			}
		}

		info := &ReleaseInfo{
			Release: r,
		}

		if canReadActions {
			statuses, _, err := git_model.GetLatestCommitStatus(ctx, r.Repo.ID, r.Sha1, db.ListOptionsAll)
			if err != nil {
				return nil, err
			}

			info.CommitStatus = git_model.CalcCommitStatus(statuses)
			info.CommitStatuses = statuses
		}

		releaseInfos = append(releaseInfos, info)
	}

	return releaseInfos, nil
}

// Releases render releases list page
func Releases(ctx *context.Context) {
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["Title"] = ctx.Tr("repo.release.releases")
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

	writeAccess := ctx.Repo.CanWrite(unit.TypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	releases, err := getReleaseInfos(ctx, &repo_model.FindReleasesOptions{
		ListOptions: listOptions,
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		IncludeDrafts: writeAccess,
		RepoID:        ctx.Repo.Repository.ID,
	})
	if err != nil {
		ctx.ServerError("getReleaseInfos", err)
		return
	}
	for _, rel := range releases {
		if rel.Release.IsTag && rel.Release.Title == "" {
			rel.Release.Title = rel.Release.TagName
		}
	}

	ctx.Data["Releases"] = releases

	numReleases := ctx.Data["NumReleases"].(int64)
	pager := context.NewPagination(int(numReleases), listOptions.PageSize, listOptions.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplReleasesList)
}

// TagsList render tags list page
func TagsList(ctx *context.Context) {
	ctx.Data["PageIsTagList"] = true
	ctx.Data["Title"] = ctx.Tr("repo.release.tags")
	ctx.Data["IsViewBranch"] = false
	ctx.Data["IsViewTag"] = true
	// Disable the showCreateNewBranch form in the dropdown on this page.
	ctx.Data["CanCreateBranch"] = false
	ctx.Data["HideBranchesInDropdown"] = true
	ctx.Data["CanCreateRelease"] = ctx.Repo.CanWrite(unit.TypeReleases) && !ctx.Repo.Repository.IsArchived

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

	opts := repo_model.FindReleasesOptions{
		ListOptions: listOptions,
		// for the tags list page, show all releases with real tags (having real commit-id),
		// the drafts should also be included because a real tag might be used as a draft.
		IncludeDrafts: true,
		IncludeTags:   true,
		HasSha1:       optional.Some(true),
		RepoID:        ctx.Repo.Repository.ID,
	}

	releases, err := db.Find[repo_model.Release](ctx, opts)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	ctx.Data["Releases"] = releases

	numTags := ctx.Data["NumTags"].(int64)
	pager := context.NewPagination(int(numTags), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.Data["PageIsViewCode"] = !ctx.Repo.Repository.UnitEnabled(ctx, unit.TypeReleases)
	ctx.HTML(http.StatusOK, tplTagsList)
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

	releases, err := getReleaseInfos(ctx, &repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{Page: 1, PageSize: 1},
		RepoID:      ctx.Repo.Repository.ID,
		TagNames:    []string{ctx.Params("*")},
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		IncludeDrafts: writeAccess,
		IncludeTags:   true,
	})
	if err != nil {
		ctx.ServerError("getReleaseInfos", err)
		return
	}
	if len(releases) != 1 {
		ctx.NotFound("SingleRelease", err)
		return
	}

	release := releases[0].Release
	if release.IsTag && release.Title == "" {
		release.Title = release.TagName
	}

	ctx.Data["PageIsSingleTag"] = release.IsTag
	if release.IsTag {
		ctx.Data["Title"] = release.TagName
	} else {
		ctx.Data["Title"] = release.Title
	}

	ctx.Data["Releases"] = releases
	ctx.HTML(http.StatusOK, tplReleasesList)
}

// LatestRelease redirects to the latest release
func LatestRelease(ctx *context.Context) {
	release, err := repo_model.GetLatestReleaseByRepoID(ctx, ctx.Repo.Repository.ID)
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
		rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
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
	ctx.Data["Assignees"] = MakeSelfOnTop(ctx.Doer, assigneeUsers)

	upload.AddUploadContext(ctx, "release")

	// For New Release page
	PrepareBranchList(ctx)
	if ctx.Written() {
		return
	}

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// NewReleasePost response for creating a release
func NewReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

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

	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, form.TagName)
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

		if err = releaseservice.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo, rel, attachmentUUIDs, nil, nil); err != nil {
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
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
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
	ctx.Data["Assignees"] = MakeSelfOnTop(ctx.Doer, assigneeUsers)

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// EditReleasePost response for edit release
func EditReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true

	tagName := ctx.Params("*")
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
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
	if err = releaseservice.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo,
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
			ctx.JSONRedirect(ctx.Repo.RepoLink + "/tags")
			return
		}

		ctx.JSONRedirect(ctx.Repo.RepoLink + "/releases")
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
