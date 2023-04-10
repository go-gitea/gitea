// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/perm"
	repo_model "code.gitea.io/gitea/models/repo"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

// ToRepo converts a Repository to api.Repository
func ToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode) *api.Repository {
	return innerToRepo(ctx, repo, mode, false)
}

func innerToRepo(ctx context.Context, repo *repo_model.Repository, mode perm.AccessMode, isParent bool) *api.Repository {
	var parent *api.Repository

	cloneLink := repo.CloneLink()
	permission := &api.Permission{
		Admin: mode >= perm.AccessModeAdmin,
		Push:  mode >= perm.AccessModeWrite,
		Pull:  mode >= perm.AccessModeRead,
	}
	if !isParent {
		err := repo.GetBaseRepo(ctx)
		if err != nil {
			return nil
		}
		if repo.BaseRepo != nil {
			parent = innerToRepo(ctx, repo.BaseRepo, mode, true)
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
		allowRebaseUpdate = config.AllowRebaseUpdate
		defaultDeleteBranchAfterMerge = config.DefaultDeleteBranchAfterMerge
		defaultMergeStyle = config.GetDefaultMergeStyle()
		defaultAllowMaintainerEdit = config.DefaultAllowMaintainerEdit
	}
	hasProjects := false
	if _, err := repo.GetUnit(ctx, unit_model.TypeProjects); err == nil {
		hasProjects = true
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

	numReleases, _ := repo_model.GetReleaseCountByRepoID(ctx, repo.ID, repo_model.FindReleasesOptions{IncludeDrafts: false, IncludeTags: false})

	mirrorInterval := ""
	var mirrorUpdated time.Time
	if repo.IsMirror {
		var err error
		repo.Mirror, err = repo_model.GetMirrorByRepoID(ctx, repo.ID)
		if err == nil {
			mirrorInterval = repo.Mirror.Interval.String()
			mirrorUpdated = repo.Mirror.UpdatedUnix.AsTime()
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
		Owner:                         ToUserWithAccessMode(ctx, repo.Owner, mode),
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
		Permissions:                   permission,
		HasIssues:                     hasIssues,
		ExternalTracker:               externalTracker,
		InternalTracker:               internalTracker,
		HasWiki:                       hasWiki,
		HasProjects:                   hasProjects,
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
