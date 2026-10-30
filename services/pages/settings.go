// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
)

// SiteSettings contains the repository-controlled Pages publication settings.
type SiteSettings struct {
	Enabled      bool
	SourceType   pages_model.PublishSource
	SourceBranch string
	SourcePath   string
}

// SaveSiteSettings creates the initially closed site or updates its automatic
// source and enabled state in one serialized state change.
func SaveSiteSettings(ctx context.Context, repoID int64, settings SiteSettings) (*pages_model.Site, error) {
	if repoID <= 0 || settings.SourceType != pages_model.PublishSourceBranch && settings.SourceType != pages_model.PublishSourceArtifact {
		return nil, errors.New("SaveSiteSettings: invalid source")
	}
	if settings.SourceType != pages_model.PublishSourceBranch && settings.SourcePath != "" {
		return nil, errors.New("SaveSiteSettings: path is only valid for a branch source")
	}
	if len(settings.SourceBranch) > 255 || len(settings.SourcePath) > 1024 {
		return nil, errors.New("SaveSiteSettings: source name is too long")
	}
	if settings.SourceType == pages_model.PublishSourceBranch && (settings.SourceBranch == "" || !git.IsValidRefPattern(settings.SourceBranch)) {
		return nil, errors.New("SaveSiteSettings: invalid branch")
	}
	if settings.SourceType == pages_model.PublishSourceArtifact && settings.SourceBranch != "" && !git.IsValidRefPattern(settings.SourceBranch) {
		return nil, errors.New("SaveSiteSettings: invalid artifact source branch")
	}
	if settings.SourcePath != "" && (path.IsAbs(settings.SourcePath) ||
		path.Clean(settings.SourcePath) != settings.SourcePath ||
		strings.Contains(settings.SourcePath, "\\") ||
		strings.HasPrefix(settings.SourcePath, "../") ||
		settings.SourcePath == "..") {
		return nil, errors.New("SaveSiteSettings: invalid source directory")
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("SaveSiteSettings: repository: %w", err)
	}
	if repo.IsArchived {
		return nil, errors.New("SaveSiteSettings: archived repository")
	}
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("SaveSiteSettings: lock site: %w", err)
	}
	defer release()
	var updatedSite *pages_model.Site
	err = withStateChange(ctx, func(ctx context.Context, updated int64) error {
		current, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil {
			return err
		}
		if !has {
			current = &pages_model.Site{RepoID: repoID}
		}
		if current.Deleted {
			return ErrStateChanged
		}
		sourceChanged := current.SourceType != settings.SourceType || current.SourceBranch != settings.SourceBranch || current.SourcePath != settings.SourcePath
		targetChanged := !has || current.Enabled != settings.Enabled || sourceChanged
		current.Enabled = settings.Enabled
		current.SourceType, current.SourceBranch, current.SourcePath = settings.SourceType, settings.SourceBranch, settings.SourcePath
		if sourceChanged {
			current.SourceCommitID = ""
			current.SourceNextCheckUnix = 0
			current.SourceErrorCode = ""
		}
		if targetChanged {
			current.PagesUpdatedUnixNano = updated
		}
		updatedSite = current
		if has {
			return pages_model.UpdateSite(ctx, current)
		}
		return pages_model.InsertSite(ctx, current)
	})
	if err != nil {
		return nil, fmt.Errorf("SaveSiteSettings: %w", err)
	}
	return updatedSite, nil
}
