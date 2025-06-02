// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/renderhelper"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/feed"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/context/upload"
	"code.gitea.io/gitea/services/forms"
	release_service "code.gitea.io/gitea/services/release"
)

const (
	tplReleasesList templates.TplName = "repo/release/list"
	tplReleaseNew   templates.TplName = "repo/release/new"
	tplTagsList     templates.TplName = "repo/tag/list"
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

		rctx := renderhelper.NewRenderContextRepoComment(ctx, r.Repo)
		r.RenderedNote, err = markdown.RenderString(rctx, r.Note)
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
			statuses, err := git_model.GetLatestCommitStatus(ctx, r.Repo.ID, r.Sha1, db.ListOptionsAll)
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
	pager.AddParamFromRequest(ctx.Req)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplReleasesList)
}

// TagsList render tags list page
func TagsList(ctx *context.Context) {
	ctx.Data["PageIsTagList"] = true
	ctx.Data["Title"] = ctx.Tr("repo.release.tags")
	ctx.Data["CanCreateRelease"] = ctx.Repo.CanWrite(unit.TypeReleases) && !ctx.Repo.Repository.IsArchived

	namePattern := ctx.FormTrim("q")

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
		NamePattern:   optional.Some(namePattern),
	}

	releases, err := db.Find[repo_model.Release](ctx, opts)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	count, err := db.Count[repo_model.Release](ctx, opts)
	if err != nil {
		ctx.ServerError("GetReleasesByRepoID", err)
		return
	}

	ctx.Data["Keyword"] = namePattern
	ctx.Data["Releases"] = releases
	ctx.Data["TagCount"] = count

	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.AddParamFromRequest(ctx.Req)
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

	writeAccess := ctx.Repo.CanWrite(unit.TypeReleases)
	ctx.Data["CanCreateRelease"] = writeAccess && !ctx.Repo.Repository.IsArchived

	releases, err := getReleaseInfos(ctx, &repo_model.FindReleasesOptions{
		ListOptions: db.ListOptions{Page: 1, PageSize: 1},
		RepoID:      ctx.Repo.Repository.ID,
		TagNames:    []string{ctx.PathParam("*")},
		// only show draft releases for users who can write, read-only users shouldn't see draft releases.
		IncludeDrafts: writeAccess,
		IncludeTags:   true,
	})
	if err != nil {
		ctx.ServerError("getReleaseInfos", err)
		return
	}
	if len(releases) != 1 {
		ctx.NotFound(err)
		return
	}

	release := releases[0].Release
	if release.IsTag && release.Title == "" {
		release.Title = release.TagName
	}

	ctx.Data["PageIsSingleTag"] = release.IsTag
	ctx.Data["SingleReleaseTagName"] = release.TagName
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
			ctx.NotFound(err)
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

func newReleaseCommon(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.new_release")
	ctx.Data["PageIsReleaseList"] = true

	tags, err := repo_model.GetTagNamesByRepoID(ctx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("GetTagNamesByRepoID", err)
		return
	}
	ctx.Data["Tags"] = tags

	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	assigneeUsers, err := repo_model.GetRepoAssignees(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoAssignees", err)
		return
	}
	ctx.Data["Assignees"] = shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers)

	upload.AddUploadContext(ctx, "release")

	PrepareBranchList(ctx) // for New Release page
}

// NewRelease render creating or edit release page
func NewRelease(ctx *context.Context) {
	newReleaseCommon(ctx)
	if ctx.Written() {
		return
	}

	ctx.Data["ShowCreateTagOnlyButton"] = true

	// pre-fill the form with the tag name, target branch and the existing release (if exists)
	ctx.Data["tag_target"] = ctx.Repo.Repository.DefaultBranch
	if tagName := ctx.FormString("tag"); tagName != "" {
		rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
		if err != nil && !repo_model.IsErrReleaseNotExist(err) {
			ctx.ServerError("GetRelease", err)
			return
		}

		if rel != nil {
			rel.Repo = ctx.Repo.Repository
			if err = rel.LoadAttributes(ctx); err != nil {
				ctx.ServerError("LoadAttributes", err)
				return
			}

			ctx.Data["ShowCreateTagOnlyButton"] = false
			ctx.Data["tag_name"] = rel.TagName
			ctx.Data["tag_target"] = rel.Target
			ctx.Data["title"] = rel.Title
			ctx.Data["content"] = rel.Note
			ctx.Data["attachments"] = rel.Attachments
		}
	}

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// NewReleasePost response for creating a release
func NewReleasePost(ctx *context.Context) {
	newReleaseCommon(ctx)
	if ctx.Written() {
		return
	}

	form := web.GetForm(ctx).(*forms.NewReleaseForm)

	// first, check whether the release exists, and prepare "ShowCreateTagOnlyButton"
	// the logic should be done before the form error check to make the tmpl has correct variables
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, form.TagName)
	if err != nil && !repo_model.IsErrReleaseNotExist(err) {
		ctx.ServerError("GetRelease", err)
		return
	}

	// We should still show the "tag only" button if the user clicks it, no matter the release exists or not.
	// Because if error occurs, end users need to have the chance to edit the name and submit the form with "tag-only" again.
	// It is still not completely right, because there could still be cases like this:
	// * user visit "new release" page, see the "tag only" button
	// * input something, click other buttons but not "tag only"
	// * error occurs, the "new release" page is rendered again, but the "tag only" button is gone
	// Such cases are not able to be handled by current code, it needs frontend code to toggle the "tag-only" button if the input changes.
	// Or another choice is "always show the tag-only button" if error occurs.
	ctx.Data["ShowCreateTagOnlyButton"] = form.TagOnly || rel == nil

	// do some form checks
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplReleaseNew)
		return
	}

	if !gitrepo.IsBranchExist(ctx, ctx.Repo.Repository, form.Target) {
		ctx.RenderWithErr(ctx.Tr("form.target_branch_not_exist"), tplReleaseNew, &form)
		return
	}

	if !form.TagOnly && form.Title == "" {
		// if not "tag only", then the title of the release cannot be empty
		ctx.RenderWithErr(ctx.Tr("repo.release.title_empty"), tplReleaseNew, &form)
		return
	}

	handleTagReleaseError := func(err error) {
		ctx.Data["Err_TagName"] = true
		switch {
		case release_service.IsErrTagAlreadyExists(err):
			ctx.RenderWithErr(ctx.Tr("repo.branch.tag_collision", form.TagName), tplReleaseNew, &form)
		case repo_model.IsErrReleaseAlreadyExist(err):
			ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
		case release_service.IsErrInvalidTagName(err):
			ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_invalid"), tplReleaseNew, &form)
		case release_service.IsErrProtectedTagName(err):
			ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_protected"), tplReleaseNew, &form)
		default:
			ctx.ServerError("handleTagReleaseError", err)
		}
	}

	// prepare the git message for creating a new tag
	newTagMsg := ""
	if form.Title != "" && form.AddTagMsg {
		newTagMsg = form.Title + "\n\n" + form.Content
	}

	// no release, and tag only
	if rel == nil && form.TagOnly {
		if err = release_service.CreateNewTag(ctx, ctx.Doer, ctx.Repo.Repository, form.Target, form.TagName, newTagMsg); err != nil {
			handleTagReleaseError(err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.tag.create_success", form.TagName))
		ctx.Redirect(ctx.Repo.RepoLink + "/src/tag/" + util.PathEscapeSegments(form.TagName))
		return
	}

	attachmentUUIDs := util.Iif(setting.Attachment.Enabled, form.Files, nil)

	// no existing release, create a new release
	if rel == nil {
		rel = &repo_model.Release{
			RepoID:       ctx.Repo.Repository.ID,
			Repo:         ctx.Repo.Repository,
			PublisherID:  ctx.Doer.ID,
			Publisher:    ctx.Doer,
			Title:        form.Title,
			TagName:      form.TagName,
			Target:       form.Target,
			Note:         form.Content,
			IsDraft:      form.Draft,
			IsPrerelease: form.Prerelease,
			IsTag:        false,
		}
		if err = release_service.CreateRelease(ctx.Repo.GitRepo, rel, attachmentUUIDs, newTagMsg); err != nil {
			handleTagReleaseError(err)
			return
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/releases")
		return
	}

	// tag exists, try to convert it to a real release
	// old logic: if the release is not a tag (it is a real release), do not update it on the "new release" page
	// add new logic: if tag-only, do not convert the tag to a release
	if form.TagOnly || !rel.IsTag {
		ctx.Data["Err_TagName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.release.tag_name_already_exist"), tplReleaseNew, &form)
		return
	}

	// convert a tag to a real release (set is_tag=false)
	rel.Title = form.Title
	rel.Note = form.Content
	rel.Target = form.Target
	rel.IsDraft = form.Draft
	rel.IsPrerelease = form.Prerelease
	rel.PublisherID = ctx.Doer.ID
	rel.IsTag = false
	if err = release_service.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo, rel, attachmentUUIDs, nil, nil); err != nil {
		handleTagReleaseError(err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/releases")
}

// EditRelease render release edit page
func EditRelease(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true
	ctx.Data["IsAttachmentEnabled"] = setting.Attachment.Enabled
	upload.AddUploadContext(ctx, "release")

	tagName := ctx.PathParam("*")
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound(err)
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
	ctx.Data["Assignees"] = shared_user.MakeSelfOnTop(ctx.Doer, assigneeUsers)

	ctx.HTML(http.StatusOK, tplReleaseNew)
}

// EditReleasePost response for edit release
func EditReleasePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.EditReleaseForm)
	ctx.Data["Title"] = ctx.Tr("repo.release.edit_release")
	ctx.Data["PageIsReleaseList"] = true
	ctx.Data["PageIsEditRelease"] = true

	tagName := ctx.PathParam("*")
	rel, err := repo_model.GetRelease(ctx, ctx.Repo.Repository.ID, tagName)
	if err != nil {
		if repo_model.IsErrReleaseNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetRelease", err)
		}
		return
	}
	if rel.IsTag {
		ctx.NotFound(err)
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
	if err = release_service.UpdateRelease(ctx, ctx.Doer, ctx.Repo.GitRepo,
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
			ctx.NotFound(err)
		} else {
			ctx.Flash.Error("DeleteReleaseByID: " + err.Error())
			redirect()
		}
		return
	}

	if err := release_service.DeleteReleaseByID(ctx, ctx.Repo.Repository, rel, ctx.Doer, isDelTag); err != nil {
		if release_service.IsErrProtectedTagName(err) {
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
