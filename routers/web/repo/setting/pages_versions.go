// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	actions_model "gitea.dev/models/actions"
	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/log"
	setting_module "gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/timeutil"
	"gitea.dev/services/context"
	"gitea.dev/services/context/upload"
	pages_service "gitea.dev/services/pages"
)

const (
	tplPagesVersions        templates.TplName = "repo/settings/pages_versions"
	pagesUploadFormOverhead                   = 1 * 1024 * 1024
)

// PagesUploadFormLimit runs before the common web context, which parses
// multipart forms before the repository upload handler receives the request.
func PagesUploadFormLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		limit := setting_module.Pages.MaxTarGzSize + pagesUploadFormOverhead
		if req.ContentLength > limit {
			http.Error(w, "Pages upload exceeds the size limit", http.StatusRequestEntityTooLarge)
			return
		}
		req.Body = http.MaxBytesReader(w, req.Body, limit)
		if strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data") {
			if err := req.ParseMultipartForm(32 * 1024 * 1024); err != nil {
				if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
					http.Error(w, "Pages upload exceeds the size limit", http.StatusRequestEntityTooLarge)
				} else {
					http.Error(w, "Invalid Pages upload form", http.StatusBadRequest)
				}
				return
			}
			defer func() {
				if err := req.MultipartForm.RemoveAll(); err != nil {
					log.Warn("Unable to remove Pages upload temporary files: %v", err)
				}
			}()
		}
		next.ServeHTTP(w, req)
	})
}

type pagesVersionView struct {
	ID          int64
	Digest      string
	ShortDigest string
	Origin      string
	OriginURL   string
	CreatedUnix timeutil.TimeStamp
	Current     bool
}

func loadPagesVersionsData(ctx *context.Context) bool {
	upload.AddUploadContext(ctx, "pages")
	repoID := ctx.Repo.Repository.ID
	site, hasSite, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		ctx.ServerError("GetSiteByRepoID", err)
		return false
	}
	if !hasSite {
		site = &pages_model.Site{RepoID: repoID}
	}

	page := max(ctx.FormInt("page"), 1)
	versions, total, err := pages_model.FindVersions(ctx, &pages_model.FindVersionOptions{
		ListOptions: db.ListOptions{Page: page, PageSize: setting_module.UI.ExplorePagingNum},
		RepoID:      repoID,
	})
	if err != nil {
		ctx.ServerError("FindVersions", err)
		return false
	}
	actionRunIDs := make([]int64, 0, len(versions))
	for _, version := range versions {
		if version.OriginActionRunID != 0 {
			actionRunIDs = append(actionRunIDs, version.OriginActionRunID)
		}
	}
	actionRuns, err := actions_model.GetRunsByRepoAndID(ctx, repoID, actionRunIDs)
	if err != nil {
		ctx.ServerError("GetRunsByRepoAndID", err)
		return false
	}
	actionRunByID := make(map[int64]*actions_model.ActionRun, len(actionRuns))
	for _, run := range actionRuns {
		actionRunByID[run.ID] = run
	}
	versionViews := make([]*pagesVersionView, 0, len(versions))
	for _, version := range versions {
		shortDigest := version.TarGzSHA256
		if len(shortDigest) > 12 {
			shortDigest = shortDigest[:12]
		}
		view := &pagesVersionView{
			ID: version.ID, Digest: version.TarGzSHA256, ShortDigest: shortDigest,
			Origin:      ctx.Locale.TrString("repo.settings.pages.versions.origin." + string(version.OriginType)),
			CreatedUnix: version.CreatedUnix, Current: version.ID == site.CurrentVersionID,
		}
		if run := actionRunByID[version.OriginActionRunID]; run != nil {
			view.OriginURL = ctx.Repo.RepoLink + "/actions/runs/" + strconv.FormatInt(run.ID, 10)
		}
		versionViews = append(versionViews, view)
	}

	ctx.Data["Title"] = ctx.Tr("repo.settings.pages.versions.title")
	ctx.Data["PageIsSettingsPages"] = true
	ctx.Data["PageIsSettingsPagesVersions"] = true
	ctx.Data["PagesVersions"] = versionViews
	ctx.Data["Total"] = total
	ctx.Data["Page"] = context.NewPagerBuilder(ctx).
		TotalCount(total).
		PerPageLimit(setting_module.UI.ExplorePagingNum).
		CurPage(page).
		Build()
	return true
}

// PagesVersions shows retained Pages versions and the archive upload control.
func PagesVersions(ctx *context.Context) {
	if !loadPagesVersionsData(ctx) {
		return
	}
	ctx.HTML(http.StatusOK, tplPagesVersions)
}

// PagesUploadArchive temporarily stores one archive selected through Gitea's upload component.
func PagesUploadArchive(ctx *context.Context) {
	file, _, err := ctx.Req.FormFile("file")
	if err != nil {
		ctx.HTTPError(http.StatusBadRequest, "Pages archive is required")
		return
	}
	defer file.Close()

	uploaded, err := repo_model.NewUpload(ctx, "pages.tar.gz", nil, file)
	if err != nil {
		ctx.ServerError("NewUpload", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]string{"uuid": uploaded.UUID})
}

// PagesRemoveUpload removes an archive that has not been submitted.
func PagesRemoveUpload(ctx *context.Context) {
	if err := repo_model.DeleteUploadByUUID(ctx, ctx.FormString("file")); err != nil {
		ctx.ServerError("DeleteUploadByUUID", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// PagesVersionsPost uploads, restores, or deletes a retained Pages version.
func PagesVersionsPost(ctx *context.Context) {
	repoID := ctx.Repo.Repository.ID
	switch ctx.FormString("action") {
	case "upload":
		files := ctx.FormStrings("files")
		if len(files) != 1 {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
			break
		}
		uploaded, err := repo_model.GetUploadByUUID(ctx, files[0])
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
			break
		}
		defer func() {
			if err := repo_model.DeleteUploads(ctx, uploaded); err != nil {
				log.Error("Unable to remove submitted Pages upload: %v", err)
			}
		}()
		file, err := os.Open(uploaded.LocalPath())
		if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
			break
		}
		defer file.Close()
		if _, err := pages_service.CommitUploadContent(ctx, repoID, file); err != nil {
			log.Warn("CommitUploadContent [repo_id: %d]: %v", repoID, err)
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.upload_error"))
			break
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.pages.upload_success"))
	case "restore-version":
		if _, err := pages_service.RestoreVersion(ctx, repoID, ctx.FormInt64("version_id")); err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.versions.restore_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.versions.restore_success"))
		}
	case "delete-version":
		err := pages_service.DeleteVersion(ctx, repoID, ctx.FormInt64("version_id"))
		if errors.Is(err, pages_service.ErrCurrentVersion) {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.versions.delete_current"))
		} else if err != nil {
			ctx.Flash.Error(ctx.Tr("repo.settings.pages.versions.delete_error"))
		} else {
			ctx.Flash.Success(ctx.Tr("repo.settings.pages.versions.delete_success"))
		}
	default:
		ctx.Flash.Error(ctx.Tr("repo.settings.pages.invalid_action"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/pages/versions")
}
