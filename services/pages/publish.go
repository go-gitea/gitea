// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

var ErrSourceChanged = errors.New("Pages source changed while publishing")

// ContentCommitResult identifies the retained version and synchronization time
// produced by a content operation.
type ContentCommitResult struct {
	Version              *pages_model.Version
	PagesUpdatedUnixNano int64
}

type contentCommitOptions struct {
	Origin         pages_model.VersionOrigin
	RequiredSource pages_model.PublishSource
	InitialSource  pages_model.PublishSource
	ActionRunID    int64
	ValidateSource func(context.Context, *pages_model.Site) error
	AcceptVersion  func(context.Context, *pages_model.Version) error
}

func archiveLimits() pages_content.Limits {
	limits, err := pages_content.NewLimits(
		setting.Pages.MaxTarGzSize,
		setting.Pages.MaxSiteSize,
		setting.Pages.MaxFileSize,
		setting.Pages.MaxEntries,
	)
	if err != nil {
		panic(err) // settings validation guarantees these values are valid
	}
	return limits
}

func lockSiteState(ctx context.Context, repoID int64) (globallock.ReleaseFunc, error) {
	return globallock.Lock(ctx, "pages_site_"+strconv.FormatInt(repoID, 10))
}

// commitContent validates and records a TAR.GZ as the repository's current
// Pages version. Source-specific facts are rechecked while the repository
// publish lock is held.
func commitContent(ctx context.Context, repoID int64, input io.Reader, opts contentCommitOptions) (*ContentCommitResult, error) {
	if repoID <= 0 || input == nil || opts.Origin == "" {
		return nil, errors.New("commitContent: invalid input")
	}
	candidate, cleanup, err := setting.AppDataTempDir("pages").CreateTempFileRandom("candidate-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("commitContent: candidate: %w", err)
	}
	defer cleanup()
	limits := archiveLimits()
	n, err := io.Copy(candidate, io.LimitReader(input, limits.MaxCompressedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("commitContent: receive: %w", err)
	}
	if n > limits.MaxCompressedBytes {
		return nil, pages_content.ErrArchiveTooLarge
	}
	if _, err := candidate.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("commitContent: rewind: %w", err)
	}
	manifest, err := pages_content.ValidateArchive(ctx, candidate, limits)
	if err != nil {
		return nil, fmt.Errorf("commitContent: validate: %w", err)
	}

	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("commitContent: lock site: %w", err)
	}
	defer release()
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("commitContent: site: %w", err)
	}
	if !has {
		if opts.InitialSource == "" {
			return nil, ErrSourceChanged
		}
		site = &pages_model.Site{RepoID: repoID, SourceType: opts.InitialSource}
	} else if site.Deleted || opts.RequiredSource != "" && (!site.Enabled || site.SourceType != opts.RequiredSource) {
		return nil, ErrSourceChanged
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, ErrSourceChanged
	}
	if repo.IsArchived {
		return nil, ErrSourceChanged
	}
	if opts.ValidateSource != nil {
		if err := opts.ValidateSource(ctx, site); err != nil {
			return nil, fmt.Errorf("commitContent: source check: %w", err)
		}
	}
	store := pages_content.NewContentStore()
	if _, err := candidate.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("commitContent: rewind for storage: %w", err)
	}
	if err := store.Save(repoID, manifest.SHA256Hex, candidate, n); err != nil {
		return nil, fmt.Errorf("commitContent: storage save: %w", err)
	}
	var version *pages_model.Version
	var pruned []*pages_model.Version
	err = withStateChange(ctx, func(ctx context.Context, updated int64) error {
		current, hasSite, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil {
			return err
		}
		if !hasSite {
			if opts.InitialSource == "" {
				return ErrSourceChanged
			}
			current = &pages_model.Site{RepoID: repoID, SourceType: opts.InitialSource}
		} else if current.Deleted || opts.RequiredSource != "" && (!current.Enabled || current.SourceType != opts.RequiredSource) {
			return ErrSourceChanged
		}
		currentRepo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil || currentRepo.IsArchived {
			return ErrSourceChanged
		}
		var hasVersion bool
		version, hasVersion, err = pages_model.GetVersionByRepoIDAndDigest(ctx, repoID, manifest.SHA256Hex)
		if err != nil {
			return err
		}
		if !hasVersion {
			version = &pages_model.Version{
				RepoID: repoID, TarGzSHA256: manifest.SHA256Hex,
				OriginType: opts.Origin, OriginActionRunID: opts.ActionRunID,
			}
			if err := pages_model.InsertVersion(ctx, version); err != nil {
				return err
			}
		}
		if current.CurrentVersionID != version.ID {
			current.CurrentVersionID = version.ID
			current.PagesUpdatedUnixNano = updated
			if hasSite {
				err = pages_model.UpdateSite(ctx, current)
			} else {
				err = pages_model.InsertSite(ctx, current)
			}
			if err != nil {
				return err
			}
		}
		if opts.AcceptVersion != nil {
			if err := opts.AcceptVersion(ctx, version); err != nil {
				return err
			}
		}
		site = current
		pruned, err = pruneVersions(ctx, repoID, current.CurrentVersionID)
		return err
	})
	if err != nil {
		if cleanupErr := deleteUnreferencedContent(ctx, store, repoID, manifest.SHA256Hex); cleanupErr != nil {
			log.Warn("Unable to clean unpublished Pages object for repo %d: %v", repoID, cleanupErr)
		}
		return nil, fmt.Errorf("commitContent: record version: %w", err)
	}
	deleteVersionObjects(ctx, store, pruned)
	return &ContentCommitResult{Version: version, PagesUpdatedUnixNano: site.PagesUpdatedUnixNano}, nil
}

func pruneVersions(ctx context.Context, repoID, currentVersionID int64) ([]*pages_model.Version, error) {
	versions, _, err := pages_model.FindVersions(ctx, &pages_model.FindVersionOptions{
		ListOptions: db.ListOptionsAll,
		RepoID:      repoID,
	})
	if err != nil {
		return nil, err
	}
	keep := setting.Pages.MaxVersionsPerSite
	kept := map[int64]struct{}{currentVersionID: {}}
	for _, version := range versions {
		if len(kept) >= keep {
			break
		}
		kept[version.ID] = struct{}{}
	}
	remove := make([]*pages_model.Version, 0, max(0, len(versions)-len(kept)))
	for _, version := range versions {
		if _, ok := kept[version.ID]; ok {
			continue
		}
		remove = append(remove, version)
	}
	if len(remove) == 0 {
		return remove, nil
	}
	ids := make([]int64, 0, len(remove))
	for _, version := range remove {
		ids = append(ids, version.ID)
	}
	_, err = db.GetEngine(ctx).In("id", ids).Delete(new(pages_model.Version))
	return remove, err
}

func deleteVersionObjects(ctx context.Context, store *pages_content.ContentStore, versions []*pages_model.Version) {
	for _, version := range versions {
		if err := deleteUnreferencedContent(ctx, store, version.RepoID, version.TarGzSHA256); err != nil {
			log.Warn("Unable to remove expired Pages object for repo %d: %v", version.RepoID, err)
		}
	}
}

func deleteUnreferencedContent(ctx context.Context, store *pages_content.ContentStore, repoID int64, digest string) error {
	_, has, err := pages_model.GetVersionByRepoIDAndDigest(ctx, repoID, digest)
	if err != nil || has {
		return err
	}
	return store.Delete(repoID, digest)
}

// RestoreVersion switches a repository to an existing retained Pages version.
func RestoreVersion(ctx context.Context, repoID, versionID int64) (*ContentCommitResult, error) {
	if repoID <= 0 || versionID <= 0 {
		return nil, errors.New("RestoreVersion: invalid version")
	}
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("RestoreVersion: lock site: %w", err)
	}
	defer release()
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("RestoreVersion: repository: %w", err)
	}
	if repo.IsArchived {
		return nil, errors.New("RestoreVersion: repository is archived")
	}
	site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
	if err != nil {
		return nil, fmt.Errorf("RestoreVersion: site: %w", err)
	}
	if !has || site.Deleted {
		return nil, util.ErrNotExist
	}
	version, has, err := pages_model.GetVersionByID(ctx, versionID)
	if err != nil || !has || version.RepoID != repoID {
		return nil, util.ErrNotExist
	}
	store := pages_content.NewContentStore()
	if _, err := store.Stat(repoID, version.TarGzSHA256); err != nil {
		return nil, fmt.Errorf("RestoreVersion: content: %w", err)
	}
	var pruned []*pages_model.Version
	err = withStateChange(ctx, func(ctx context.Context, updated int64) error {
		current, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil || !has || current.Deleted {
			return ErrStateChanged
		}
		currentRepo, err := repo_model.GetRepositoryByID(ctx, repoID)
		if err != nil || currentRepo.IsArchived {
			return ErrStateChanged
		}
		if current.CurrentVersionID != version.ID {
			current.CurrentVersionID = version.ID
			current.PagesUpdatedUnixNano = updated
			if err := pages_model.UpdateSite(ctx, current); err != nil {
				return err
			}
		}
		site = current
		pruned, err = pruneVersions(ctx, repoID, current.CurrentVersionID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("RestoreVersion: %w", err)
	}
	deleteVersionObjects(ctx, store, pruned)
	return &ContentCommitResult{Version: version, PagesUpdatedUnixNano: site.PagesUpdatedUnixNano}, nil
}

// DeleteVersion removes one retained version and its unreferenced archive.
func DeleteVersion(ctx context.Context, repoID, versionID int64) error {
	if repoID <= 0 || versionID <= 0 {
		return errors.New("DeleteVersion: invalid version")
	}
	repo, err := repo_model.GetRepositoryByID(ctx, repoID)
	if err != nil {
		return fmt.Errorf("DeleteVersion: repository: %w", err)
	}
	if repo.IsArchived {
		return errors.New("DeleteVersion: repository is archived")
	}
	release, err := lockSiteState(ctx, repoID)
	if err != nil {
		return fmt.Errorf("DeleteVersion: lock site: %w", err)
	}
	defer release()

	var deleted *pages_model.Version
	err = withStateChange(ctx, func(ctx context.Context, updated int64) error {
		site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
		if err != nil {
			return err
		}
		if !has || site.Deleted {
			return util.ErrNotExist
		}
		version, has, err := pages_model.GetVersionByID(ctx, versionID)
		if err != nil {
			return err
		}
		if !has || version.RepoID != repoID {
			return util.ErrNotExist
		}
		if site.CurrentVersionID == versionID {
			site.CurrentVersionID = 0
			site.Enabled = false
			site.PagesUpdatedUnixNano = updated
			if err := pages_model.UpdateSite(ctx, site); err != nil {
				return err
			}
		}
		if err := pages_model.DeleteVersionByID(ctx, versionID); err != nil {
			return err
		}
		deleted = version
		return nil
	})
	if err != nil {
		return fmt.Errorf("DeleteVersion: %w", err)
	}
	if err := deleteUnreferencedContent(ctx, pages_content.NewContentStore(), repoID, deleted.TarGzSHA256); err != nil {
		log.Warn("Unable to remove deleted Pages object for repo %d: %v", repoID, err)
	}
	return nil
}

// CommitUploadContent accepts a direct upload as the current retained version.
// The first upload initializes a closed artifact-backed site. Existing sites
// retain their automatic publish source and its progress cursor.
func CommitUploadContent(ctx context.Context, repoID int64, input io.Reader) (*ContentCommitResult, error) {
	return commitContent(ctx, repoID, input, contentCommitOptions{
		Origin: pages_model.VersionOriginUpload, InitialSource: pages_model.PublishSourceArtifact,
	})
}
