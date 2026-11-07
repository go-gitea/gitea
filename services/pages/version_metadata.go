// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/util"
)

// VersionMetadataUpdate contains mutable metadata for a retained version.
type VersionMetadataUpdate struct {
	Comment     *string
	KeepForever *bool
}

// UpdateVersionMetadata changes metadata without publishing the version.
func UpdateVersionMetadata(ctx context.Context, repoID, versionID int64, update VersionMetadataUpdate) (*pages_model.Version, error) {
	if repoID <= 0 || versionID <= 0 || update.Comment == nil && update.KeepForever == nil {
		return nil, util.NewInvalidArgumentErrorf("invalid Pages version metadata update")
	}
	if update.Comment != nil {
		if strings.ContainsAny(*update.Comment, "\r\n") {
			return nil, util.NewInvalidArgumentErrorf("Pages version comment must be one line")
		}
		comment := strings.TrimSpace(*update.Comment)
		if utf8.RuneCountInString(comment) > pages_model.MaxVersionCommentLength {
			return nil, util.NewInvalidArgumentErrorf("Pages version comment is too long")
		}
		update.Comment = &comment
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("UpdateVersionMetadata: repository: %w", err)
	}
	if repo.IsArchived {
		return nil, errors.New("UpdateVersionMetadata: repository is archived")
	}
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("UpdateVersionMetadata: lock site: %w", err)
	}
	defer release()

	var version *pages_model.Version
	err = db.WithTx(ctx, func(ctx context.Context) error {
		var has bool
		version, has, err = pages_model.GetVersionByID(ctx, versionID)
		if err != nil {
			return err
		}
		if !has || version.RepoID != repoID {
			return util.ErrNotExist
		}
		if update.Comment != nil {
			version.Comment = *update.Comment
		}
		if update.KeepForever != nil {
			version.KeepForever = *update.KeepForever
		}
		return pages_model.UpdateVersionMetadata(ctx, version)
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateVersionMetadata: %w", err)
	}
	return version, nil
}
