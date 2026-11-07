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

// SiteSettings contains repository-controlled Pages settings.
type SiteSettings struct {
	Enabled               bool
	SourceType            pages_model.PublishSource
	SourceBranch          string
	SourcePath            string
	VersionRetentionCount *int
}

// SaveSiteSettings creates the initially closed site or updates its automatic
// source and enabled state in one serialized state change.
func SaveSiteSettings(ctx context.Context, repoID int64, settings SiteSettings) (*pages_model.Site, error) {
	if repoID <= 0 {
		return nil, errors.New("SaveSiteSettings: invalid repository")
	}
	if err := validateSiteSettings(settings); err != nil {
		return nil, fmt.Errorf("SaveSiteSettings: %w", err)
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
			current = &pages_model.Site{RepoID: repoID, VersionRetentionCount: pages_model.DefaultVersionRetentionCount}
		}
		if current.Deleted {
			return ErrStateChanged
		}
		sourceChanged := current.SourceType != settings.SourceType || current.SourceBranch != settings.SourceBranch || current.SourcePath != settings.SourcePath
		targetChanged := !has || current.Enabled != settings.Enabled || sourceChanged
		current.Enabled = settings.Enabled
		current.SourceType, current.SourceBranch, current.SourcePath = settings.SourceType, settings.SourceBranch, settings.SourcePath
		if settings.VersionRetentionCount != nil {
			current.VersionRetentionCount = *settings.VersionRetentionCount
		}
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

func validateSiteSettings(settings SiteSettings) error {
	if settings.SourceType != pages_model.PublishSourceBranch && settings.SourceType != pages_model.PublishSourceArtifact {
		return errors.New("invalid source")
	}
	if settings.SourceType != pages_model.PublishSourceBranch && settings.SourcePath != "" {
		return errors.New("path is only valid for a branch source")
	}
	if len(settings.SourceBranch) > 255 || len(settings.SourcePath) > 1024 {
		return errors.New("source name is too long")
	}
	if settings.SourceType == pages_model.PublishSourceBranch && (settings.SourceBranch == "" || !git.IsValidRefPattern(settings.SourceBranch)) {
		return errors.New("invalid branch")
	}
	if settings.SourceType == pages_model.PublishSourceArtifact && settings.SourceBranch != "" && !git.IsValidRefPattern(settings.SourceBranch) {
		return errors.New("invalid artifact source branch")
	}
	if settings.SourcePath != "" && (path.IsAbs(settings.SourcePath) ||
		path.Clean(settings.SourcePath) != settings.SourcePath ||
		strings.Contains(settings.SourcePath, "\\") ||
		strings.HasPrefix(settings.SourcePath, "../") ||
		settings.SourcePath == "..") {
		return errors.New("invalid source directory")
	}
	if settings.VersionRetentionCount != nil && !pages_model.IsValidVersionRetentionCount(*settings.VersionRetentionCount) {
		return errors.New("invalid version retention count")
	}
	return nil
}
