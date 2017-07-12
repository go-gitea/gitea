// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	api "code.gitea.io/sdk/gitea"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

// GetRelease get a single release of a repository
func GetRelease(ctx *context.APIContext) {
	id := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}
	if err := release.LoadAttributes(); err != nil {
		ctx.Error(500, "LoadAttributes", err)
		return
	}
	ctx.JSON(200, release.APIFormat())
}

// ListReleases list a repository's releases
func ListReleases(ctx *context.APIContext) {
	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, models.FindReleasesOptions{
		IncludeDrafts: ctx.Repo.AccessMode >= models.AccessModeWrite,
	}, 1, 2147483647)
	if err != nil {
		ctx.Error(500, "GetReleasesByRepoID", err)
		return
	}
	rels := make([]*api.Release, len(releases))
	for i, release := range releases {
		if err := release.LoadAttributes(); err != nil {
			ctx.Error(500, "LoadAttributes", err)
			return
		}
		rels[i] = release.APIFormat()
	}
	ctx.JSON(200, rels)
}

// CreateRelease create a release
func CreateRelease(ctx *context.APIContext, form api.CreateReleaseOption) {
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	if !ctx.Repo.GitRepo.IsTagExist(form.TagName) {
		ctx.Status(404)
		return
	}
	tag, err := ctx.Repo.GitRepo.GetTag(form.TagName)
	if err != nil {
		ctx.Error(500, "GetTag", err)
		return
	}
	commit, err := tag.Commit()
	if err != nil {
		ctx.Error(500, "Commit", err)
		return
	}
	commitsCount, err := commit.CommitsCount()
	if err != nil {
		ctx.Error(500, "CommitsCount", err)
		return
	}
	rel := &models.Release{
		RepoID:       ctx.Repo.Repository.ID,
		PublisherID:  ctx.User.ID,
		Publisher:    ctx.User,
		TagName:      form.TagName,
		LowerTagName: strings.ToLower(form.TagName),
		Target:       form.Target,
		Title:        form.Title,
		Sha1:         commit.ID.String(),
		NumCommits:   commitsCount,
		Note:         form.Note,
		IsDraft:      form.IsDraft,
		IsPrerelease: form.IsPrerelease,
		CreatedUnix:  commit.Author.When.Unix(),
	}
	if err := models.CreateRelease(ctx.Repo.GitRepo, rel, nil); err != nil {
		if models.IsErrReleaseAlreadyExist(err) {
			ctx.Status(409)
		} else {
			ctx.Error(500, "CreateRelease", err)
		}
		return
	}
	ctx.JSON(201, rel.APIFormat())
}

// EditRelease edit a release
func EditRelease(ctx *context.APIContext, form api.EditReleaseOption) {
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	id := ctx.ParamsInt64(":id")
	rel, err := models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if rel.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}

	if len(form.TagName) > 0 {
		rel.TagName = form.TagName
	}
	if len(form.Target) > 0 {
		rel.Target = form.Target
	}
	if len(form.Title) > 0 {
		rel.Title = form.Title
	}
	if len(form.Note) > 0 {
		rel.Note = form.Note
	}
	if form.IsDraft != nil {
		rel.IsDraft = *form.IsDraft
	}
	if form.IsPrerelease != nil {
		rel.IsPrerelease = *form.IsPrerelease
	}
	if err := models.UpdateRelease(ctx.Repo.GitRepo, rel, nil); err != nil {
		ctx.Error(500, "UpdateRelease", err)
		return
	}

	rel, err = models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if err := rel.LoadAttributes(); err != nil {
		ctx.Error(500, "LoadAttributes", err)
		return
	}
	ctx.JSON(200, rel.APIFormat())
}

// DeleteRelease delete a release from a repository
func DeleteRelease(ctx *context.APIContext) {
	if ctx.Repo.AccessMode < models.AccessModeWrite {
		ctx.Status(403)
		return
	}
	id := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(500, "GetReleaseByID", err)
		return
	}
	if release.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(404)
		return
	}
	if err := models.DeleteReleaseByID(id, ctx.User, false); err != nil {
		ctx.Error(500, "DeleteReleaseByID", err)
		return
	}
	ctx.Status(204)
}
