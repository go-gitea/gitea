// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/indexer/code"
	"code.gitea.io/gitea/modules/indexer/stats"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	mirror_module "code.gitea.io/gitea/modules/mirror"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	asymkey_service "code.gitea.io/gitea/services/asymkey"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
	"code.gitea.io/gitea/services/migrations"
	mirror_service "code.gitea.io/gitea/services/mirror"
	org_service "code.gitea.io/gitea/services/org"
	repo_service "code.gitea.io/gitea/services/repository"
	wiki_service "code.gitea.io/gitea/services/wiki"
)

const (
	tplSettingsOptions base.TplName = "repo/settings/options"
	tplCollaboration   base.TplName = "repo/settings/collaboration"
	tplBranches        base.TplName = "repo/settings/branches"
	tplTags            base.TplName = "repo/settings/tags"
	tplGithooks        base.TplName = "repo/settings/githooks"
	tplGithookEdit     base.TplName = "repo/settings/githook_edit"
	tplDeployKeys      base.TplName = "repo/settings/deploy_keys"
)

// SettingsCtxData is a middleware that sets all the general context data for the
// settings template.
func SettingsCtxData(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.options")
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.Data["ForcePrivate"] = setting.Repository.ForcePrivate
	ctx.Data["MirrorsEnabled"] = setting.Mirror.Enabled
	ctx.Data["DisableNewPushMirrors"] = setting.Mirror.DisableNewPush
	ctx.Data["DefaultMirrorInterval"] = setting.Mirror.DefaultInterval
	ctx.Data["MinimumMirrorInterval"] = setting.Mirror.MinInterval

	signing, _ := asymkey_service.SigningKey(ctx, ctx.Repo.Repository.RepoPath())
	ctx.Data["SigningKeyAvailable"] = len(signing) > 0
	ctx.Data["SigningSettings"] = setting.Repository.Signing
	ctx.Data["CodeIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	if ctx.Doer.IsAdmin {
		if setting.Indexer.RepoIndexerEnabled {
			status, err := repo_model.GetIndexerStatus(ctx, ctx.Repo.Repository, repo_model.RepoIndexerTypeCode)
			if err != nil {
				ctx.ServerError("repo.indexer_status", err)
				return
			}
			ctx.Data["CodeIndexerStatus"] = status
		}
		status, err := repo_model.GetIndexerStatus(ctx, ctx.Repo.Repository, repo_model.RepoIndexerTypeStats)
		if err != nil {
			ctx.ServerError("repo.indexer_status", err)
			return
		}
		ctx.Data["StatsIndexerStatus"] = status
	}
	pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(ctx, ctx.Repo.Repository.ID, db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetPushMirrorsByRepoID", err)
		return
	}
	ctx.Data["PushMirrors"] = pushMirrors
}

// Settings show a repository's settings page
func Settings(ctx *context.Context) {
	ctx.HTML(http.StatusOK, tplSettingsOptions)
}

// SettingsPost response for changes of a repository
func SettingsPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.RepoSettingForm)

	ctx.Data["ForcePrivate"] = setting.Repository.ForcePrivate
	ctx.Data["MirrorsEnabled"] = setting.Mirror.Enabled
	ctx.Data["DisableNewPushMirrors"] = setting.Mirror.DisableNewPush
	ctx.Data["DefaultMirrorInterval"] = setting.Mirror.DefaultInterval
	ctx.Data["MinimumMirrorInterval"] = setting.Mirror.MinInterval

	signing, _ := asymkey_service.SigningKey(ctx, ctx.Repo.Repository.RepoPath())
	ctx.Data["SigningKeyAvailable"] = len(signing) > 0
	ctx.Data["SigningSettings"] = setting.Repository.Signing
	ctx.Data["CodeIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled

	repo := ctx.Repo.Repository

	switch ctx.FormString("action") {
	case "update":
		if ctx.HasError() {
			ctx.HTML(http.StatusOK, tplSettingsOptions)
			return
		}

		newRepoName := form.RepoName
		// Check if repository name has been changed.
		if repo.LowerName != strings.ToLower(newRepoName) {
			// Close the GitRepo if open
			if ctx.Repo.GitRepo != nil {
				ctx.Repo.GitRepo.Close()
				ctx.Repo.GitRepo = nil
			}
			if err := repo_service.ChangeRepositoryName(ctx, ctx.Doer, repo, newRepoName); err != nil {
				ctx.Data["Err_RepoName"] = true
				switch {
				case repo_model.IsErrRepoAlreadyExist(err):
					ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tplSettingsOptions, &form)
				case db.IsErrNameReserved(err):
					ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(db.ErrNameReserved).Name), tplSettingsOptions, &form)
				case repo_model.IsErrRepoFilesAlreadyExist(err):
					ctx.Data["Err_RepoName"] = true
					switch {
					case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
						ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"), tplSettingsOptions, form)
					case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
						ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt"), tplSettingsOptions, form)
					case setting.Repository.AllowDeleteOfUnadoptedRepositories:
						ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.delete"), tplSettingsOptions, form)
					default:
						ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist"), tplSettingsOptions, form)
					}
				case db.IsErrNamePatternNotAllowed(err):
					ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(db.ErrNamePatternNotAllowed).Pattern), tplSettingsOptions, &form)
				default:
					ctx.ServerError("ChangeRepositoryName", err)
				}
				return
			}

			log.Trace("Repository name changed: %s/%s -> %s", ctx.Repo.Owner.Name, repo.Name, newRepoName)
		}
		// In case it's just a case change.
		repo.Name = newRepoName
		repo.LowerName = strings.ToLower(newRepoName)
		repo.Description = form.Description
		repo.Website = form.Website
		repo.IsTemplate = form.Template

		// Visibility of forked repository is forced sync with base repository.
		if repo.IsFork {
			form.Private = repo.BaseRepo.IsPrivate || repo.BaseRepo.Owner.Visibility == structs.VisibleTypePrivate
		}

		visibilityChanged := repo.IsPrivate != form.Private
		// when ForcePrivate enabled, you could change public repo to private, but only admin users can change private to public
		if visibilityChanged && setting.Repository.ForcePrivate && !form.Private && !ctx.Doer.IsAdmin {
			ctx.RenderWithErr(ctx.Tr("form.repository_force_private"), tplSettingsOptions, form)
			return
		}

		repo.IsPrivate = form.Private
		if err := repo_service.UpdateRepository(ctx, repo, visibilityChanged); err != nil {
			ctx.ServerError("UpdateRepository", err)
			return
		}
		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "mirror":
		if !setting.Mirror.Enabled || !repo.IsMirror {
			ctx.NotFound("", nil)
			return
		}

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		interval, err := time.ParseDuration(form.Interval)
		if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
			ctx.Data["Err_Interval"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_interval_invalid"), tplSettingsOptions, &form)
			return
		}

		ctx.Repo.Mirror.EnablePrune = form.EnablePrune
		ctx.Repo.Mirror.Interval = interval
		ctx.Repo.Mirror.ScheduleNextUpdate()
		if err := repo_model.UpdateMirror(ctx, ctx.Repo.Mirror); err != nil {
			ctx.ServerError("UpdateMirror", err)
			return
		}

		u, err := git.GetRemoteURL(ctx, ctx.Repo.Repository.RepoPath(), ctx.Repo.Mirror.GetRemoteName())
		if err != nil {
			ctx.Data["Err_MirrorAddress"] = true
			handleSettingRemoteAddrError(ctx, err, form)
			return
		}
		if u.User != nil && form.MirrorPassword == "" && form.MirrorUsername == u.User.Username() {
			form.MirrorPassword, _ = u.User.Password()
		}

		address, err := forms.ParseRemoteAddr(form.MirrorAddress, form.MirrorUsername, form.MirrorPassword)
		if err == nil {
			err = migrations.IsMigrateURLAllowed(address, ctx.Doer)
		}
		if err != nil {
			ctx.Data["Err_MirrorAddress"] = true
			handleSettingRemoteAddrError(ctx, err, form)
			return
		}

		if err := mirror_service.UpdateAddress(ctx, ctx.Repo.Mirror, address); err != nil {
			ctx.ServerError("UpdateAddress", err)
			return
		}

		form.LFS = form.LFS && setting.LFS.StartServer

		if len(form.LFSEndpoint) > 0 {
			ep := lfs.DetermineEndpoint("", form.LFSEndpoint)
			if ep == nil {
				ctx.Data["Err_LFSEndpoint"] = true
				ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_lfs_endpoint"), tplSettingsOptions, &form)
				return
			}
			err = migrations.IsMigrateURLAllowed(ep.String(), ctx.Doer)
			if err != nil {
				ctx.Data["Err_LFSEndpoint"] = true
				handleSettingRemoteAddrError(ctx, err, form)
				return
			}
		}

		ctx.Repo.Mirror.LFS = form.LFS
		ctx.Repo.Mirror.LFSEndpoint = form.LFSEndpoint
		if err := repo_model.UpdateMirror(ctx, ctx.Repo.Mirror); err != nil {
			ctx.ServerError("UpdateMirror", err)
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "mirror-sync":
		if !setting.Mirror.Enabled || !repo.IsMirror {
			ctx.NotFound("", nil)
			return
		}

		mirror_module.AddPullMirrorToQueue(repo.ID)

		ctx.Flash.Info(ctx.Tr("repo.settings.mirror_sync_in_progress"))
		ctx.Redirect(repo.Link() + "/settings")

	case "push-mirror-sync":
		if !setting.Mirror.Enabled {
			ctx.NotFound("", nil)
			return
		}

		m, err := selectPushMirrorByForm(ctx, form, repo)
		if err != nil {
			ctx.NotFound("", nil)
			return
		}

		mirror_module.AddPushMirrorToQueue(m.ID)

		ctx.Flash.Info(ctx.Tr("repo.settings.mirror_sync_in_progress"))
		ctx.Redirect(repo.Link() + "/settings")

	case "push-mirror-remove":
		if !setting.Mirror.Enabled {
			ctx.NotFound("", nil)
			return
		}

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		m, err := selectPushMirrorByForm(ctx, form, repo)
		if err != nil {
			ctx.NotFound("", nil)
			return
		}

		if err = mirror_service.RemovePushMirrorRemote(ctx, m); err != nil {
			ctx.ServerError("RemovePushMirrorRemote", err)
			return
		}

		if err = repo_model.DeletePushMirrors(ctx, repo_model.PushMirrorOptions{ID: m.ID, RepoID: m.RepoID}); err != nil {
			ctx.ServerError("DeletePushMirrorByID", err)
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "push-mirror-add":
		if setting.Mirror.DisableNewPush {
			ctx.NotFound("", nil)
			return
		}

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		interval, err := time.ParseDuration(form.PushMirrorInterval)
		if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
			ctx.Data["Err_PushMirrorInterval"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_interval_invalid"), tplSettingsOptions, &form)
			return
		}

		address, err := forms.ParseRemoteAddr(form.PushMirrorAddress, form.PushMirrorUsername, form.PushMirrorPassword)
		if err == nil {
			err = migrations.IsMigrateURLAllowed(address, ctx.Doer)
		}
		if err != nil {
			ctx.Data["Err_PushMirrorAddress"] = true
			handleSettingRemoteAddrError(ctx, err, form)
			return
		}

		remoteSuffix, err := util.CryptoRandomString(10)
		if err != nil {
			ctx.ServerError("RandomString", err)
			return
		}

		m := &repo_model.PushMirror{
			RepoID:       repo.ID,
			Repo:         repo,
			RemoteName:   fmt.Sprintf("remote_mirror_%s", remoteSuffix),
			SyncOnCommit: form.PushMirrorSyncOnCommit,
			Interval:     interval,
		}
		if err := repo_model.InsertPushMirror(ctx, m); err != nil {
			ctx.ServerError("InsertPushMirror", err)
			return
		}

		if err := mirror_service.AddPushMirrorRemote(ctx, m, address); err != nil {
			if err := repo_model.DeletePushMirrors(ctx, repo_model.PushMirrorOptions{ID: m.ID, RepoID: m.RepoID}); err != nil {
				log.Error("DeletePushMirrors %v", err)
			}
			ctx.ServerError("AddPushMirrorRemote", err)
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "advanced":
		var repoChanged bool
		var units []repo_model.RepoUnit
		var deleteUnitTypes []unit_model.Type

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		if repo.CloseIssuesViaCommitInAnyBranch != form.EnableCloseIssuesViaCommitInAnyBranch {
			repo.CloseIssuesViaCommitInAnyBranch = form.EnableCloseIssuesViaCommitInAnyBranch
			repoChanged = true
		}

		if form.EnableCode && !unit_model.TypeCode.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeCode,
			})
		} else if !unit_model.TypeCode.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeCode)
		}

		if form.EnableWiki && form.EnableExternalWiki && !unit_model.TypeExternalWiki.UnitGlobalDisabled() {
			if !validation.IsValidExternalURL(form.ExternalWikiURL) {
				ctx.Flash.Error(ctx.Tr("repo.settings.external_wiki_url_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}

			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeExternalWiki,
				Config: &repo_model.ExternalWikiConfig{
					ExternalWikiURL: form.ExternalWikiURL,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeWiki)
		} else if form.EnableWiki && !form.EnableExternalWiki && !unit_model.TypeWiki.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeWiki,
				Config: new(repo_model.UnitConfig),
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalWiki)
		} else {
			if !unit_model.TypeExternalWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalWiki)
			}
			if !unit_model.TypeWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeWiki)
			}
		}

		if form.EnableIssues && form.EnableExternalTracker && !unit_model.TypeExternalTracker.UnitGlobalDisabled() {
			if !validation.IsValidExternalURL(form.ExternalTrackerURL) {
				ctx.Flash.Error(ctx.Tr("repo.settings.external_tracker_url_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}
			if len(form.TrackerURLFormat) != 0 && !validation.IsValidExternalTrackerURLFormat(form.TrackerURLFormat) {
				ctx.Flash.Error(ctx.Tr("repo.settings.tracker_url_format_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeExternalTracker,
				Config: &repo_model.ExternalTrackerConfig{
					ExternalTrackerURL:           form.ExternalTrackerURL,
					ExternalTrackerFormat:        form.TrackerURLFormat,
					ExternalTrackerStyle:         form.TrackerIssueStyle,
					ExternalTrackerRegexpPattern: form.ExternalTrackerRegexpPattern,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeIssues)
		} else if form.EnableIssues && !form.EnableExternalTracker && !unit_model.TypeIssues.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeIssues,
				Config: &repo_model.IssuesConfig{
					EnableTimetracker:                form.EnableTimetracker,
					AllowOnlyContributorsToTrackTime: form.AllowOnlyContributorsToTrackTime,
					EnableDependencies:               form.EnableIssueDependencies,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalTracker)
		} else {
			if !unit_model.TypeExternalTracker.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeExternalTracker)
			}
			if !unit_model.TypeIssues.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeIssues)
			}
		}

		if form.EnableProjects && !unit_model.TypeProjects.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeProjects,
			})
		} else if !unit_model.TypeProjects.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeProjects)
		}

		if form.EnableReleases && !unit_model.TypeReleases.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeReleases,
			})
		} else if !unit_model.TypeReleases.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeReleases)
		}

		if form.EnablePackages && !unit_model.TypePackages.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypePackages,
			})
		} else if !unit_model.TypePackages.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypePackages)
		}

		if form.EnableActions && !unit_model.TypeActions.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypeActions,
			})
		} else if !unit_model.TypeActions.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypeActions)
		}

		if form.EnablePulls && !unit_model.TypePullRequests.UnitGlobalDisabled() {
			units = append(units, repo_model.RepoUnit{
				RepoID: repo.ID,
				Type:   unit_model.TypePullRequests,
				Config: &repo_model.PullRequestsConfig{
					IgnoreWhitespaceConflicts:     form.PullsIgnoreWhitespace,
					AllowMerge:                    form.PullsAllowMerge,
					AllowRebase:                   form.PullsAllowRebase,
					AllowRebaseMerge:              form.PullsAllowRebaseMerge,
					AllowSquash:                   form.PullsAllowSquash,
					AllowManualMerge:              form.PullsAllowManualMerge,
					AutodetectManualMerge:         form.EnableAutodetectManualMerge,
					AllowRebaseUpdate:             form.PullsAllowRebaseUpdate,
					DefaultDeleteBranchAfterMerge: form.DefaultDeleteBranchAfterMerge,
					DefaultMergeStyle:             repo_model.MergeStyle(form.PullsDefaultMergeStyle),
					DefaultAllowMaintainerEdit:    form.DefaultAllowMaintainerEdit,
				},
			})
		} else if !unit_model.TypePullRequests.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, unit_model.TypePullRequests)
		}

		if len(units) == 0 {
			ctx.Flash.Error(ctx.Tr("repo.settings.update_settings_no_unit"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		if err := repo_model.UpdateRepositoryUnits(repo, units, deleteUnitTypes); err != nil {
			ctx.ServerError("UpdateRepositoryUnits", err)
			return
		}
		if repoChanged {
			if err := repo_service.UpdateRepository(ctx, repo, false); err != nil {
				ctx.ServerError("UpdateRepository", err)
				return
			}
		}
		log.Trace("Repository advanced settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "signing":
		changed := false
		trustModel := repo_model.ToTrustModel(form.TrustModel)
		if trustModel != repo.TrustModel {
			repo.TrustModel = trustModel
			changed = true
		}

		if changed {
			if err := repo_service.UpdateRepository(ctx, repo, false); err != nil {
				ctx.ServerError("UpdateRepository", err)
				return
			}
		}
		log.Trace("Repository signing settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "admin":
		if !ctx.Doer.IsAdmin {
			ctx.Error(http.StatusForbidden)
			return
		}

		if repo.IsFsckEnabled != form.EnableHealthCheck {
			repo.IsFsckEnabled = form.EnableHealthCheck
		}

		if err := repo_service.UpdateRepository(ctx, repo, false); err != nil {
			ctx.ServerError("UpdateRepository", err)
			return
		}

		log.Trace("Repository admin settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "admin_index":
		if !ctx.Doer.IsAdmin {
			ctx.Error(http.StatusForbidden)
			return
		}

		switch form.RequestReindexType {
		case "stats":
			if err := stats.UpdateRepoIndexer(ctx.Repo.Repository); err != nil {
				ctx.ServerError("UpdateStatsRepondexer", err)
				return
			}
		case "code":
			if !setting.Indexer.RepoIndexerEnabled {
				ctx.Error(http.StatusForbidden)
				return
			}
			code.UpdateRepoIndexer(ctx.Repo.Repository)
		default:
			ctx.NotFound("", nil)
			return
		}

		log.Trace("Repository reindex for %s requested: %s/%s", form.RequestReindexType, ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.reindex_requested"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "convert":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		if !repo.IsMirror {
			ctx.Error(http.StatusNotFound)
			return
		}
		repo.IsMirror = false

		if _, err := repo_module.CleanUpMigrateInfo(ctx, repo); err != nil {
			ctx.ServerError("CleanUpMigrateInfo", err)
			return
		} else if err = repo_model.DeleteMirrorByRepoID(ctx.Repo.Repository.ID); err != nil {
			ctx.ServerError("DeleteMirrorByRepoID", err)
			return
		}
		log.Trace("Repository converted from mirror to regular: %s", repo.FullName())
		ctx.Flash.Success(ctx.Tr("repo.settings.convert_succeed"))
		ctx.Redirect(repo.Link())

	case "convert_fork":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}
		if err := repo.LoadOwner(ctx); err != nil {
			ctx.ServerError("Convert Fork", err)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		if !repo.IsFork {
			ctx.Error(http.StatusNotFound)
			return
		}

		if !ctx.Repo.Owner.CanCreateRepo() {
			maxCreationLimit := ctx.Repo.Owner.MaxCreationLimit()
			msg := ctx.TrN(maxCreationLimit, "repo.form.reach_limit_of_creation_1", "repo.form.reach_limit_of_creation_n", maxCreationLimit)
			ctx.Flash.Error(msg)
			ctx.Redirect(repo.Link() + "/settings")
			return
		}

		if err := repo_service.ConvertForkToNormalRepository(ctx, repo); err != nil {
			log.Error("Unable to convert repository %-v from fork. Error: %v", repo, err)
			ctx.ServerError("Convert Fork", err)
			return
		}

		log.Trace("Repository converted from fork to regular: %s", repo.FullName())
		ctx.Flash.Success(ctx.Tr("repo.settings.convert_fork_succeed"))
		ctx.Redirect(repo.Link())

	case "transfer":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		newOwner, err := user_model.GetUserByName(ctx, ctx.FormString("new_owner_name"))
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_owner_name"), tplSettingsOptions, nil)
				return
			}
			ctx.ServerError("IsUserExist", err)
			return
		}

		if newOwner.Type == user_model.UserTypeOrganization {
			if !ctx.Doer.IsAdmin && newOwner.Visibility == structs.VisibleTypePrivate && !organization.OrgFromUser(newOwner).HasMemberWithUserID(ctx.Doer.ID) {
				// The user shouldn't know about this organization
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_owner_name"), tplSettingsOptions, nil)
				return
			}
		}

		// Close the GitRepo if open
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
			ctx.Repo.GitRepo = nil
		}

		if err := repo_service.StartRepositoryTransfer(ctx, ctx.Doer, newOwner, repo, nil); err != nil {
			if repo_model.IsErrRepoAlreadyExist(err) {
				ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplSettingsOptions, nil)
			} else if models.IsErrRepoTransferInProgress(err) {
				ctx.RenderWithErr(ctx.Tr("repo.settings.transfer_in_progress"), tplSettingsOptions, nil)
			} else {
				ctx.ServerError("TransferOwnership", err)
			}

			return
		}

		log.Trace("Repository transfer process was started: %s/%s -> %s", ctx.Repo.Owner.Name, repo.Name, newOwner)
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer_started", newOwner.DisplayName()))
		ctx.Redirect(repo.Link() + "/settings")

	case "cancel_transfer":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}

		repoTransfer, err := models.GetPendingRepositoryTransfer(ctx, ctx.Repo.Repository)
		if err != nil {
			if models.IsErrNoPendingTransfer(err) {
				ctx.Flash.Error("repo.settings.transfer_abort_invalid")
				ctx.Redirect(repo.Link() + "/settings")
			} else {
				ctx.ServerError("GetPendingRepositoryTransfer", err)
			}

			return
		}

		if err := repoTransfer.LoadAttributes(ctx); err != nil {
			ctx.ServerError("LoadRecipient", err)
			return
		}

		if err := models.CancelRepositoryTransfer(ctx.Repo.Repository); err != nil {
			ctx.ServerError("CancelRepositoryTransfer", err)
			return
		}

		log.Trace("Repository transfer process was cancelled: %s/%s ", ctx.Repo.Owner.Name, repo.Name)
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer_abort_success", repoTransfer.Recipient.Name))
		ctx.Redirect(repo.Link() + "/settings")

	case "delete":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		// Close the gitrepository before doing this.
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
		}

		if err := repo_service.DeleteRepository(ctx, ctx.Doer, ctx.Repo.Repository, true); err != nil {
			ctx.ServerError("DeleteRepository", err)
			return
		}
		log.Trace("Repository deleted: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
		ctx.Redirect(ctx.Repo.Owner.DashboardLink())

	case "delete-wiki":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		err := wiki_service.DeleteWiki(ctx, repo)
		if err != nil {
			log.Error("Delete Wiki: %v", err.Error())
		}
		log.Trace("Repository wiki deleted: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.wiki_deletion_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "archive":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusForbidden)
			return
		}

		if repo.IsMirror {
			ctx.Flash.Error(ctx.Tr("repo.settings.archive.error_ismirror"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		if err := repo_model.SetArchiveRepoState(repo, true); err != nil {
			log.Error("Tried to archive a repo: %s", err)
			ctx.Flash.Error(ctx.Tr("repo.settings.archive.error"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.archive.success"))

		log.Trace("Repository was archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "unarchive":
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusForbidden)
			return
		}

		if err := repo_model.SetArchiveRepoState(repo, false); err != nil {
			log.Error("Tried to unarchive a repo: %s", err)
			ctx.Flash.Error(ctx.Tr("repo.settings.unarchive.error"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.unarchive.success"))

		log.Trace("Repository was un-archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	default:
		ctx.NotFound("", nil)
	}
}

func handleSettingRemoteAddrError(ctx *context.Context, err error, form *forms.RepoSettingForm) {
	if models.IsErrInvalidCloneAddr(err) {
		addrErr := err.(*models.ErrInvalidCloneAddr)
		switch {
		case addrErr.IsProtocolInvalid:
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_protocol_invalid"), tplSettingsOptions, form)
		case addrErr.IsURLError:
			ctx.RenderWithErr(ctx.Tr("form.url_error", addrErr.Host), tplSettingsOptions, form)
		case addrErr.IsPermissionDenied:
			if addrErr.LocalPath {
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tplSettingsOptions, form)
			} else {
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied_blocked"), tplSettingsOptions, form)
			}
		case addrErr.IsInvalidPath:
			ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tplSettingsOptions, form)
		default:
			ctx.ServerError("Unknown error", err)
		}
		return
	}
	ctx.RenderWithErr(ctx.Tr("repo.mirror_address_url_invalid"), tplSettingsOptions, form)
}

// Collaboration render a repository's collaboration page
func Collaboration(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.collaboration")
	ctx.Data["PageIsSettingsCollaboration"] = true

	users, err := repo_model.GetCollaborators(ctx, ctx.Repo.Repository.ID, db.ListOptions{})
	if err != nil {
		ctx.ServerError("GetCollaborators", err)
		return
	}
	ctx.Data["Collaborators"] = users

	teams, err := organization.GetRepoTeams(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.ServerError("GetRepoTeams", err)
		return
	}
	ctx.Data["Teams"] = teams
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["OrgID"] = ctx.Repo.Repository.OwnerID
	ctx.Data["OrgName"] = ctx.Repo.Repository.OwnerName
	ctx.Data["Org"] = ctx.Repo.Repository.Owner
	ctx.Data["Units"] = unit_model.Units

	ctx.HTML(http.StatusOK, tplCollaboration)
}

// CollaborationPost response for actions for a collaboration of a repository
func CollaborationPost(ctx *context.Context) {
	name := utils.RemoveUsernameParameterSuffix(strings.ToLower(ctx.FormString("collaborator")))
	if len(name) == 0 || ctx.Repo.Owner.LowerName == name {
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	u, err := user_model.GetUserByName(ctx, name)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}

	if !u.IsActive {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_inactive_user"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	// Organization is not allowed to be added as a collaborator.
	if u.IsOrganization() {
		ctx.Flash.Error(ctx.Tr("repo.settings.org_not_allowed_to_be_collaborator"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
		return
	}

	if got, err := repo_model.IsCollaborator(ctx, ctx.Repo.Repository.ID, u.ID); err == nil && got {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	// find the owner team of the organization the repo belongs too and
	// check if the user we're trying to add is an owner.
	if ctx.Repo.Repository.Owner.IsOrganization() {
		if isOwner, err := organization.IsOrganizationOwner(ctx, ctx.Repo.Repository.Owner.ID, u.ID); err != nil {
			ctx.ServerError("IsOrganizationOwner", err)
			return
		} else if isOwner {
			ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_owner"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
			return
		}
	}

	if err = repo_module.AddCollaborator(ctx, ctx.Repo.Repository, u); err != nil {
		ctx.ServerError("AddCollaborator", err)
		return
	}

	if setting.Service.EnableNotifyMail {
		mailer.SendCollaboratorMail(u, ctx.Doer, ctx.Repo.Repository)
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_collaborator_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.EscapedPath())
}

// ChangeCollaborationAccessMode response for changing access of a collaboration
func ChangeCollaborationAccessMode(ctx *context.Context) {
	if err := repo_model.ChangeCollaborationAccessMode(
		ctx,
		ctx.Repo.Repository,
		ctx.FormInt64("uid"),
		perm.AccessMode(ctx.FormInt("mode"))); err != nil {
		log.Error("ChangeCollaborationAccessMode: %v", err)
	}
}

// DeleteCollaboration delete a collaboration for a repository
func DeleteCollaboration(ctx *context.Context) {
	if err := models.DeleteCollaboration(ctx.Repo.Repository, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteCollaboration: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.remove_collaborator_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/collaboration",
	})
}

// AddTeamPost response for adding a team to a repository
func AddTeamPost(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	name := utils.RemoveUsernameParameterSuffix(strings.ToLower(ctx.FormString("team")))
	if len(name) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := organization.OrgFromUser(ctx.Repo.Owner).GetTeam(ctx, name)
	if err != nil {
		if organization.IsErrTeamNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.team_not_exist"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		} else {
			ctx.ServerError("GetTeam", err)
		}
		return
	}

	if team.OrgID != ctx.Repo.Repository.OwnerID {
		ctx.Flash.Error(ctx.Tr("repo.settings.team_not_in_organization"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if organization.HasTeamRepo(ctx, ctx.Repo.Repository.OwnerID, team.ID, ctx.Repo.Repository.ID) {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_team_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if err = org_service.TeamAddRepository(team, ctx.Repo.Repository); err != nil {
		ctx.ServerError("TeamAddRepository", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_team_success"))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
}

// DeleteTeam response for deleting a team from a repository
func DeleteTeam(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := organization.GetTeamByID(ctx, ctx.FormInt64("id"))
	if err != nil {
		ctx.ServerError("GetTeamByID", err)
		return
	}

	if err = models.RemoveRepository(team, ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("team.RemoveRepositorys", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.remove_team_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/collaboration",
	})
}

// GitHooks hooks of a repository
func GitHooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.githooks")
	ctx.Data["PageIsSettingsGitHooks"] = true

	hooks, err := ctx.Repo.GitRepo.Hooks()
	if err != nil {
		ctx.ServerError("Hooks", err)
		return
	}
	ctx.Data["Hooks"] = hooks

	ctx.HTML(http.StatusOK, tplGithooks)
}

// GitHooksEdit render for editing a hook of repository page
func GitHooksEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.githooks")
	ctx.Data["PageIsSettingsGitHooks"] = true

	name := ctx.Params(":name")
	hook, err := ctx.Repo.GitRepo.GetHook(name)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound("GetHook", err)
		} else {
			ctx.ServerError("GetHook", err)
		}
		return
	}
	ctx.Data["Hook"] = hook
	ctx.HTML(http.StatusOK, tplGithookEdit)
}

// GitHooksEditPost response for editing a git hook of a repository
func GitHooksEditPost(ctx *context.Context) {
	name := ctx.Params(":name")
	hook, err := ctx.Repo.GitRepo.GetHook(name)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound("GetHook", err)
		} else {
			ctx.ServerError("GetHook", err)
		}
		return
	}
	hook.Content = ctx.FormString("content")
	if err = hook.Update(); err != nil {
		ctx.ServerError("hook.Update", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/hooks/git")
}

// DeployKeys render the deploy keys list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys") + " / " + ctx.Tr("secrets.secrets")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	ctx.HTML(http.StatusOK, tplDeployKeys)
}

// DeployKeysPost response for adding a deploy key of a repository
func DeployKeysPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddKeyForm)
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := asymkey_model.ListDeployKeys(ctx, &asymkey_model.ListDeployKeysOptions{RepoID: ctx.Repo.Repository.ID})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplDeployKeys)
		return
	}

	content, err := asymkey_model.CheckPublicKeyString(form.Content)
	if err != nil {
		if db.IsErrSSHDisabled(err) {
			ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
		} else if asymkey_model.IsErrKeyUnableVerify(err) {
			ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
		} else if err == asymkey_model.ErrKeyIsPrivate {
			ctx.Data["HasError"] = true
			ctx.Data["Err_Content"] = true
			ctx.Flash.Error(ctx.Tr("form.must_use_public_key"))
		} else {
			ctx.Data["HasError"] = true
			ctx.Data["Err_Content"] = true
			ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
		return
	}

	key, err := asymkey_model.AddDeployKey(ctx.Repo.Repository.ID, form.Title, content, !form.IsWritable)
	if err != nil {
		ctx.Data["HasError"] = true
		switch {
		case asymkey_model.IsErrDeployKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		case asymkey_model.IsErrDeployKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	log.Trace("Deploy key added: %d", ctx.Repo.Repository.ID)
	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	if err := asymkey_service.DeleteDeployKey(ctx.Doer, ctx.FormInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/keys",
	})
}

// UpdateAvatarSetting update repo's avatar
func UpdateAvatarSetting(ctx *context.Context, form forms.AvatarForm) error {
	ctxRepo := ctx.Repo.Repository

	if form.Avatar == nil {
		// No avatar is uploaded and we not removing it here.
		// No random avatar generated here.
		// Just exit, no action.
		if ctxRepo.CustomAvatarRelativePath() == "" {
			log.Trace("No avatar was uploaded for repo: %d. Default icon will appear instead.", ctxRepo.ID)
		}
		return nil
	}

	r, err := form.Avatar.Open()
	if err != nil {
		return fmt.Errorf("Avatar.Open: %w", err)
	}
	defer r.Close()

	if form.Avatar.Size > setting.Avatar.MaxFileSize {
		return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big"))
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("io.ReadAll: %w", err)
	}
	st := typesniffer.DetectContentType(data)
	if !(st.IsImage() && !st.IsSvgImage()) {
		return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
	}
	if err = repo_service.UploadAvatar(ctx, ctxRepo, data); err != nil {
		return fmt.Errorf("UploadAvatar: %w", err)
	}
	return nil
}

// SettingsAvatar save new POSTed repository avatar
func SettingsAvatar(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
	form.Source = forms.AvatarLocal
	if err := UpdateAvatarSetting(ctx, *form); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.update_avatar_success"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}

// SettingsDeleteAvatar delete repository avatar
func SettingsDeleteAvatar(ctx *context.Context) {
	if err := repo_service.DeleteAvatar(ctx, ctx.Repo.Repository); err != nil {
		ctx.Flash.Error(fmt.Sprintf("DeleteAvatar: %v", err))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}

func selectPushMirrorByForm(ctx *context.Context, form *forms.RepoSettingForm, repo *repo_model.Repository) (*repo_model.PushMirror, error) {
	id, err := strconv.ParseInt(form.PushMirrorID, 10, 64)
	if err != nil {
		return nil, err
	}

	pushMirrors, _, err := repo_model.GetPushMirrorsByRepoID(ctx, repo.ID, db.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, m := range pushMirrors {
		if m.ID == id {
			m.Repo = repo
			return m, nil
		}
	}

	return nil, fmt.Errorf("PushMirror[%v] not associated to repository %v", id, repo)
}
