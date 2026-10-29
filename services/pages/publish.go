// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
)

var ErrSourceChanged = errors.New("Pages source changed while publishing")

type ContentCommitResult struct {
	TarGzSHA256          string
	PagesUpdatedUnixNano int64
}

func lockSiteState(ctx context.Context, repoID int64) (globallock.ReleaseFunc, error) {
	return globallock.Lock(ctx, "pages_site_"+strconv.FormatInt(repoID, 10))
}

// CommitContent makes a checked TAR.GZ the repository's one current object.
// sourceCheck revalidates source-specific facts under the repository publish lock.
func CommitContent(ctx context.Context, repoID int64, sourceType pages_model.SourceType, input io.Reader, sourceCheck func(context.Context, *pages_model.Site) error) (*ContentCommitResult, error) {
	if repoID <= 0 || input == nil {
		return nil, errors.New("CommitContent: invalid input")
	}
	candidate, cleanup, err := setting.AppDataTempDir("pages").CreateTempFileRandom("candidate-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("CommitContent: candidate: %w", err)
	}
	defer cleanup()
	n, err := io.Copy(candidate, io.LimitReader(input, pages_content.DefaultMaxCompressedSize+1))
	if err != nil {
		return nil, fmt.Errorf("CommitContent: receive: %w", err)
	}
	if n > pages_content.DefaultMaxCompressedSize {
		return nil, pages_content.ErrArchiveTooLarge
	}
	if _, err := candidate.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("CommitContent: rewind: %w", err)
	}
	manifest, err := pages_content.ValidateArchive(ctx, candidate, pages_content.DefaultLimits())
	if err != nil {
		return nil, fmt.Errorf("CommitContent: validate: %w", err)
	}

	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("CommitContent: lock site: %w", err)
	}
	defer release()
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("CommitContent: site: %w", err)
	}
	if !has || site.Deleted || site.SourceType != sourceType {
		return nil, ErrSourceChanged
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, ErrSourceChanged
	}
	if repo.IsArchived {
		return nil, ErrSourceChanged
	}
	if sourceCheck != nil {
		if err := sourceCheck(ctx, site); err != nil {
			return nil, fmt.Errorf("CommitContent: source check: %w", err)
		}
	}
	store := pages_content.NewContentStore()
	if _, err := candidate.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("CommitContent: rewind for storage: %w", err)
	}
	if err := store.Save(repoID, manifest.SHA256Hex, candidate, n); err != nil {
		return nil, fmt.Errorf("CommitContent: storage save: %w", err)
	}
	if site.TarGzSHA256 != manifest.SHA256Hex {
		oldDigest := site.TarGzSHA256
		err := withStateChange(ctx, func(ctx context.Context, updated int64) error {
			current, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			if !has || current.Deleted || current.SourceType != sourceType {
				return ErrSourceChanged
			}
			current.TarGzSHA256 = manifest.SHA256Hex
			current.PagesUpdatedUnixNano = updated
			site = current
			return pages_model.UpsertSite(ctx, current)
		})
		if err != nil {
			// A transaction can commit even when its response is lost. Read the
			// current fact before considering the candidate unreferenced.
			current, has, lookupErr := pages_model.GetSiteByRepoID(ctx, repoID)
			if lookupErr == nil && has && current.TarGzSHA256 == manifest.SHA256Hex {
				return &ContentCommitResult{TarGzSHA256: manifest.SHA256Hex, PagesUpdatedUnixNano: current.PagesUpdatedUnixNano}, nil
			}
			if lookupErr == nil {
				if deleteErr := store.Delete(repoID, manifest.SHA256Hex); deleteErr != nil {
					log.Warn("Unable to clean unpublished Pages object for repo %d: %v", repoID, deleteErr)
				}
			}
			return nil, fmt.Errorf("CommitContent: update current digest: %w", err)
		}
		if oldDigest != "" {
			if err := store.Delete(repoID, oldDigest); err != nil {
				log.Warn("Unable to remove old Pages object for repo %d: %v", repoID, err)
			}
		}
	}
	return &ContentCommitResult{TarGzSHA256: manifest.SHA256Hex, PagesUpdatedUnixNano: site.PagesUpdatedUnixNano}, nil
}

// CommitUploadContent accepts a direct upload from a repository that has not
// configured Pages yet. The upload selects the upload source and leaves the
// site closed until the repository settings explicitly enable it. Existing
// sites keep their selected source, so an upload cannot silently replace a
// branch or Actions deployment.
func CommitUploadContent(ctx context.Context, repoID int64, input io.Reader) (*ContentCommitResult, error) {
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("CommitUploadContent: site: %w", err)
	}
	if !has {
		if _, err := SaveSiteSettings(ctx, repoID, SiteSettings{SourceType: pages_model.SourceTypeUpload}); err != nil {
			return nil, fmt.Errorf("CommitUploadContent: initialize site: %w", err)
		}
	} else if site.Deleted || site.SourceType != pages_model.SourceTypeUpload {
		return nil, ErrSourceChanged
	}
	return CommitContent(ctx, repoID, pages_model.SourceTypeUpload, input, nil)
}

type SiteSettings struct {
	Enabled      bool
	SourceType   pages_model.SourceType
	SourceBranch string
	SourcePath   string
}

// SaveSiteSettings creates the initially closed site or updates one current
// source and public-access choice in a single transaction.
func SaveSiteSettings(ctx context.Context, repoID int64, settings SiteSettings) (*pages_model.Site, error) {
	if repoID <= 0 || settings.SourceType != pages_model.SourceTypeBranch && settings.SourceType != pages_model.SourceTypeActions && settings.SourceType != pages_model.SourceTypeUpload {
		return nil, errors.New("SaveSiteSettings: invalid source")
	}
	if settings.SourceType != pages_model.SourceTypeBranch && settings.SourcePath != "" {
		return nil, errors.New("SaveSiteSettings: path is only valid for a branch source")
	}
	if len(settings.SourceBranch) > 255 || len(settings.SourcePath) > 1024 {
		return nil, errors.New("SaveSiteSettings: source name is too long")
	}
	if settings.SourceType == pages_model.SourceTypeBranch && (settings.SourceBranch == "" || !git.IsValidRefPattern(settings.SourceBranch)) {
		return nil, errors.New("SaveSiteSettings: invalid branch")
	}
	if settings.SourceType == pages_model.SourceTypeActions && settings.SourceBranch != "" && !git.IsValidRefPattern(settings.SourceBranch) {
		return nil, errors.New("SaveSiteSettings: invalid Actions source branch")
	}
	if settings.SourceType == pages_model.SourceTypeUpload && settings.SourceBranch != "" {
		return nil, errors.New("SaveSiteSettings: upload source has no branch")
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
		if err := pages_model.UpsertSite(ctx, current); err != nil {
			return err
		}
		updatedSite = current
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("SaveSiteSettings: %w", err)
	}
	return updatedSite, nil
}

// DisableSite changes an existing repository's public site to a closed state.
func DisableSite(ctx context.Context, repoID int64) error {
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return fmt.Errorf("DisableSite: lock site: %w", err)
	}
	defer release()
	return withStateChange(ctx, func(ctx context.Context, updated int64) error {
		site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil || !has || site.Deleted || !site.Enabled {
			return err
		}
		site.Enabled = false
		site.PagesUpdatedUnixNano = updated
		return pages_model.UpsertSite(ctx, site)
	})
}
