// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(ctx context.Context, repo *repo_model.Repository, permissionInRepo access_model.Permission) *api.Repository {
	return innerToRepo(ctx, repo, permissionInRepo, false)
}

func innerToRepo(ctx context.Context, repo *repo_model.Repository, permissionInRepo access_model.Permission, isParent bool) *api.Repository {
	var parent *api.Repository

	if !permissionInRepo.HasUnits() && permissionInRepo.AccessMode > perm.AccessModeNone {
		// If units is empty, it means that it's a hard-coded permission, like access_model.Permission{AccessMode: perm.AccessModeAdmin}
		// So we need to load units for the repo, otherwise UnitAccessMode will just return perm.AccessModeNone.
		// TODO: this logic is still not right (because unit modes are not correctly prepared)
		//   the caller should prepare a proper "permission" before calling this function.
		_ = repo.LoadUnits(ctx) // the error is not important, so ignore it
		permissionInRepo.SetUnitsWithDefaultAccessMode(repo.Units, permissionInRepo.AccessMode)
	}

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: permissionInRepo.AccessMode >= perm.AccessModeAdmin,
		Push:  permissionInRepo.UnitAccessMode(unit_model.TypeCode) >= perm.AccessModeWrite,
		Pull:  permissionInRepo.UnitAccessMode(unit_model.TypeCode) >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo(ctx)
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			// FIXME: The permission of the parent repo is not correct.
			//        It's the permission of the current repo, so it's probably different from the parent repo.
			//        But there isn't a good way to get the permission of the parent repo, because the doer is not passed in.
			//        Use the permission of the current repo to keep the behavior consistent with the old API.
			//        Maybe the right way is setting the permission of the parent repo to nil, empty is better than wrong.
			parent = innerToRepo(ctx, repo.BaseRepo, permissionInRepo, true)
		}
	}

	// check enabled/disabled units
	hasIssues := false
	var externalTracker *api.ExternalTracker
	var internalTracker *api.InternalTracker
	if unit, err := repo.GetUnit(ctx, unit_model.TypeIssues); err == nil {
		config := unit.IssuesConfig()
		hasIssues = true
		internalTracker = &api.InternalTracker{
			EnableTimeTracker:                config.EnableTimetracker,
			AllowOnlyContributorsToTrackTime: config.AllowOnlyContributorsToTrackTime,
			EnableIssueDependencies:          config.EnableDependencies,
		}
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalTracker); err == nil {
		config := unit.ExternalTrackerConfig()
		hasIssues = true
		externalTracker = &api.ExternalTracker{
			ExternalTrackerURL:           config.ExternalTrackerURL,
			ExternalTrackerFormat:        config.ExternalTrackerFormat,
			ExternalTrackerStyle:         config.ExternalTrackerStyle,
			ExternalTrackerRegexpPattern: config.ExternalTrackerRegexpPattern,
		}
	}
	hasWiki := false
	var externalWiki *api.ExternalWiki
	if _, err := repo.GetUnit(ctx, unit_model.TypeWiki); err == nil {
		hasWiki = true
	} else if unit, err := repo.GetUnit(ctx, unit_model.TypeExternalWiki); err == nil {
		hasWiki = true
		config := unit.ExternalWikiConfig()
		externalWiki = &api.ExternalWiki{
			ExternalWikiURL: config.ExternalWikiURL,
		}
	}
	hasPullRequests := false
	ignoreWhitespaceConflicts := false
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquash := false
	allowFastForwardOnly := false
	allowRebaseUpdate := false
	defaultDeleteBranchAfterMerge := false
	defaultMergeStyle := repo_model.MergeStyleMerge
	defaultAllowMaintainerEdit := false
	if unit, err := repo.GetUnit(ctx, unit_model.TypePullRequests); err == nil {
		config := unit.PullRequestsConfig()
		hasPullRequests = true
		ignoreWhitespaceConflicts = config.IgnoreWhitespaceConflicts
		allowMerge = config.AllowMerge
		allowRebase = config.AllowRebase
		allowRebaseMerge = config.AllowRebaseMerge
		allowSquash = config.AllowSquash
		allowFastForwardOnly = config.AllowFastForwardOnly
		allowRebaseUpdate = config.AllowRebaseUpdate
		defaultDeleteBranchAfterMerge = config.DefaultDeleteBranchAfterMerge
		defaultMergeStyle = config.GetDefaultMergeStyle()
		defaultAllowMaintainerEdit = config.DefaultAllowMaintainerEdit
	}
	hasProjects := false
	projectsMode := repo_model.ProjectsModeAll
	if unit, err := repo.GetUnit(ctx, unit_model.TypeProjects); err == nil {
		hasProjects = true
		config := unit.ProjectsConfig()
		projectsMode = config.ProjectsMode
	}

	hasReleases := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeReleases); err == nil {
		hasReleases = true
	}

	hasPackages := false
	if _, err := repo.GetUnit(ctx, unit_model.TypePackages); err == nil {
		hasPackages = true
	}

	hasActions := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeActions); err == nil {
		hasActions = true
	}

	if err := repo.LoadOwner(ctx); err != nil {
		return nil
	}

	numReleases, _ := db.Count[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeDrafts: false,
		IncludeTags:   false,
		RepoID:        repo.ID,
	})

	mirrorInterval := ""
	var mirrorUpdated time.Time
	if repo.IsMirror {
		pullMirror, err := repo_model.GetMirrorByRepoID(ctx, repo.ID)
		if err == nil {
			mirrorInterval = pullMirror.Interval.String()
			mirrorUpdated = pullMirror.UpdatedUnix.AsTime()
		}
	}

	var transfer *api.RepoTransfer
	if repo.Status == repo_model.RepositoryPendingTransfer {
		t, err := models.GetPendingRepositoryTransfer(ctx, repo)
		if err != nil && !models.IsErrNoPendingTransfer(err) {
			log.Warn("GetPendingRepositoryTransfer: %v", err)
		} else {
			if err := t.LoadAttributes(ctx); err != nil {
				log.Warn("LoadAttributes of RepoTransfer: %v", err)
			} else {
				transfer = ToRepoTransfer(ctx, t)
			}
		}
	}

	var language string
	if repo.PrimaryLanguage != nil {
		language = repo.PrimaryLanguage.Language
	}

	repoAPIURL := repo.APIURL()

	return &api.Repository{
		ID:                            repo.ID,
		Owner:                         ToUserWithAccessMode(ctx, repo.Owner, permissionInRepo.AccessMode),
		Name:                          repo.Name,
		FullName:                      repo.FullName(),
		Description:                   repo.Description,
		Private:                       repo.IsPrivate,
		Template:                      repo.IsTemplate,
		Empty:                         repo.IsEmpty,
		Archived:                      repo.IsArchived,
		Size:                          int(repo.Size / 1024),
		Fork:                          repo.IsFork,
		Parent:                        parent,
		Mirror:                        repo.IsMirror,
		HTMLURL:                       repo.HTMLURL(),
		URL:                           repoAPIURL,
		SSHURL:                        cloneLink.SSH,
		CloneURL:                      cloneLink.HTTPS,
		OriginalURL:                   repo.SanitizedOriginalURL(),
		Website:                       repo.Website,
		Language:                      language,
		LanguagesURL:                  repoAPIURL + "/languages",
		Stars:                         repo.NumStars,
		Forks:                         repo.NumForks,
		Watchers:                      repo.NumWatches,
		OpenIssues:                    repo.NumOpenIssues,
		OpenPulls:                     repo.NumOpenPulls,
		Releases:                      int(numReleases),
		DefaultBranch:                 repo.DefaultBranch,
		Created:                       repo.CreatedUnix.AsTime(),
		Updated:                       repo.UpdatedUnix.AsTime(),
		ArchivedAt:                    repo.ArchivedUnix.AsTime(),
		Permissions:                   permission,
		HasIssues:                     hasIssues,
		ExternalTracker:               externalTracker,
		InternalTracker:               internalTracker,
		HasWiki:                       hasWiki,
		HasProjects:                   hasProjects,
		ProjectsMode:                  string(projectsMode),
		HasReleases:                   hasReleases,
		HasPackages:                   hasPackages,
		HasActions:                    hasActions,
		ExternalWiki:                  externalWiki,
		HasPullRequests:               hasPullRequests,
		IgnoreWhitespaceConflicts:     ignoreWhitespaceConflicts,
		AllowMerge:                    allowMerge,
		AllowRebase:                   allowRebase,
		AllowRebaseMerge:              allowRebaseMerge,
		AllowSquash:                   allowSquash,
		AllowFastForwardOnly:          allowFastForwardOnly,
		AllowRebaseUpdate:             allowRebaseUpdate,
		DefaultDeleteBranchAfterMerge: defaultDeleteBranchAfterMerge,
		DefaultMergeStyle:             string(defaultMergeStyle),
		DefaultAllowMaintainerEdit:    defaultAllowMaintainerEdit,
		AvatarURL:                     repo.AvatarLink(ctx),
		Internal:                      !repo.IsPrivate && repo.Owner.Visibility == api.VisibleTypePrivate,
		MirrorInterval:                mirrorInterval,
		MirrorUpdated:                 mirrorUpdated,
		RepoTransfer:                  transfer,
	}
}

// ToRepoTransfer convert a models.RepoTransfer to a structs.RepeTransfer
func ToRepoTransfer(ctx context.Context, t *models.RepoTransfer) *api.RepoTransfer {
	teams, _ := ToTeams(ctx, t.Teams, false)

	return &api.RepoTransfer{
		Doer:      ToUser(ctx, t.Doer, nil),
		Recipient: ToUser(ctx, t.Recipient, nil),
		Teams:     teams,
	}
}
