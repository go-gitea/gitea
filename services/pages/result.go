// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	pages_content "gitea.dev/modules/pages"
	"gitea.dev/modules/util"
)

var ErrInvalidResult = util.NewInvalidArgumentErrorf("invalid Pages instance result")

type ResultRequest struct {
	RepoID                          int64                         `json:"repo_id"`
	AppliedPagesUpdatedUnixNano     int64                         `json:"applied_pages_updated_unix_nano"`
	AppliedRootUpdatedUnixNano      int64                         `json:"applied_root_updated_unix_nano"`
	AppliedTXTPolicyUpdatedUnixNano int64                         `json:"applied_txt_policy_updated_unix_nano"`
	ConfigApplied                   bool                          `json:"config_applied"`
	ActualTarGzSHA256               string                        `json:"actual_tar_gz_sha256"`
	DefaultPublished                bool                          `json:"default_published"`
	CustomPublished                 bool                          `json:"custom_published"`
	ErrorCode                       pages_model.InstanceErrorCode `json:"error_code"`
}

func validResult(result ResultRequest) bool {
	if result.RepoID <= 0 || result.AppliedPagesUpdatedUnixNano <= 0 || result.AppliedRootUpdatedUnixNano <= 0 ||
		result.AppliedTXTPolicyUpdatedUnixNano < 0 || len(result.ErrorCode) > 64 {
		return false
	}
	if !result.ConfigApplied && (result.DefaultPublished || result.CustomPublished) {
		return false
	}
	if result.ActualTarGzSHA256 != "" {
		if _, err := pages_content.ContentRelativePath(result.RepoID, result.ActualTarGzSHA256); err != nil {
			return false
		}
	}
	switch result.ErrorCode {
	case "", pages_model.InstanceErrorCertificatePending, pages_model.InstanceErrorContentDownloadFailed,
		pages_model.InstanceErrorContentInvalid, pages_model.InstanceErrorContentInstallFailed, pages_model.InstanceErrorRouteApplyFailed,
		pages_model.InstanceErrorTXTInvalid, pages_model.InstanceErrorACMEFailed, pages_model.InstanceErrorEntryProbeFailed:
		return true
	default:
		return false
	}
}

// AcceptInstanceResult stores only an instance result that still matches current Gitea facts.
func AcceptInstanceResult(ctx context.Context, instanceID int64, tokenHash string, result ResultRequest) error {
	if !validResult(result) {
		return ErrInvalidResult
	}
	var release bool
	err := db.WithTx(ctx, func(ctx context.Context) error {
		// These writes provide row locks on each supported database before the
		// current facts are read. A no-op update still obtains the write lock.
		if _, err := db.Exec(ctx, "UPDATE pages_instance SET token_hash = token_hash WHERE id = ? AND token_hash = ?", instanceID, tokenHash); err != nil {
			return err
		}
		instance, has, err := pages_model.GetInstanceByID(ctx, instanceID)
		if err != nil {
			return err
		}
		if !has || !util.CryptoConstTimeEqual(instance.TokenHash, tokenHash) || instance.RootUpdatedUnixNano != result.AppliedRootUpdatedUnixNano {
			return ErrStateChanged
		}
		if _, err := db.Exec(ctx, "UPDATE pages_site SET pages_updated_unix_nano = pages_updated_unix_nano WHERE repo_id = ? AND pages_updated_unix_nano = ?", result.RepoID, result.AppliedPagesUpdatedUnixNano); err != nil {
			return err
		}
		site, has, err := pages_model.GetSiteByRepoID(ctx, result.RepoID)
		if err != nil {
			return err
		}
		if !has || site.PagesUpdatedUnixNano != result.AppliedPagesUpdatedUnixNano {
			return ErrStateChanged
		}
		if (result.DefaultPublished || result.CustomPublished) &&
			(site.Deleted || !site.Enabled || site.TarGzSHA256 == "" || result.ActualTarGzSHA256 != site.TarGzSHA256) {
			return fmt.Errorf("%w: published content does not match current content", ErrInvalidResult)
		}
		domains, err := pages_model.ListDomainsByRepoID(ctx, result.RepoID)
		if err != nil {
			return err
		}
		active, releasing := false, false
		for _, domain := range domains {
			active = active || domain.State == pages_model.DomainStateActive
			releasing = releasing || domain.State == pages_model.DomainStateReleasing
		}
		if site.Deleted {
			if result.AppliedTXTPolicyUpdatedUnixNano != 0 {
				return ErrStateChanged
			}
			if !releasing {
				return nil
			}
		} else if active {
			_, policyTime, err := GetTXTVerificationPolicy(ctx)
			if err != nil {
				return err
			}
			if policyTime != result.AppliedTXTPolicyUpdatedUnixNano {
				return ErrStateChanged
			}
		} else if result.AppliedTXTPolicyUpdatedUnixNano != 0 {
			return ErrStateChanged
		}
		if err := pages_model.UpsertInstanceResult(ctx, &pages_model.InstanceResult{
			InstanceID: instanceID, RepoID: result.RepoID,
			AppliedPagesUpdatedUnixNano:     result.AppliedPagesUpdatedUnixNano,
			AppliedRootUpdatedUnixNano:      result.AppliedRootUpdatedUnixNano,
			AppliedTXTPolicyUpdatedUnixNano: result.AppliedTXTPolicyUpdatedUnixNano,
			ConfigApplied:                   result.ConfigApplied,
			ActualTarGzSHA256:               result.ActualTarGzSHA256,
			DefaultPublished:                result.DefaultPublished,
			CustomPublished:                 result.CustomPublished,
			ErrorCode:                       result.ErrorCode,
		}); err != nil {
			return err
		}
		release = releasing
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrStateChanged) {
			return err
		}
		return fmt.Errorf("AcceptInstanceResult: %w", err)
	}
	if release {
		if err := ReleaseConfirmedDomains(ctx, result.RepoID); err != nil {
			return fmt.Errorf("AcceptInstanceResult: release domains: %w", err)
		}
	}
	return nil
}

// ReleaseConfirmedDomains removes old Host reservations after all registered instances
// have applied the current site and root-address facts.
func ReleaseConfirmedDomains(ctx context.Context, repoID int64) error {
	return withDomainLock(ctx, func(ctx context.Context) error {
		stateClock.Lock()
		defer stateClock.Unlock()
		return db.WithTx(ctx, func(ctx context.Context) error {
			site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			if !has {
				return nil
			}
			domains, err := pages_model.ListDomainsByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			releasing := make([]*pages_model.Domain, 0, len(domains))
			for _, domain := range domains {
				if domain.State == pages_model.DomainStateReleasing {
					releasing = append(releasing, domain)
				}
			}
			if len(releasing) == 0 {
				return nil
			}
			instances, err := pages_model.ListInstances(ctx)
			if err != nil {
				return err
			}
			for _, instance := range instances {
				if instance.RootURL == "" {
					continue
				}
				report, has, err := pages_model.GetInstanceResult(ctx, instance.ID, repoID)
				if err != nil {
					return err
				}
				if !has || !report.ConfigApplied || report.AppliedPagesUpdatedUnixNano != site.PagesUpdatedUnixNano ||
					report.AppliedRootUpdatedUnixNano != instance.RootUpdatedUnixNano {
					return nil
				}
			}
			for _, domain := range releasing {
				if err := pages_model.DeleteDomainByID(ctx, domain.ID); err != nil {
					return err
				}
			}
			if site.Deleted {
				updated, err := nextStateTime(ctx)
				if err == nil {
					site.PagesUpdatedUnixNano = updated
					err = pages_model.UpsertSite(ctx, site)
				}
				if err != nil {
					return err
				}
				if _, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).Delete(new(pages_model.InstanceResult)); err != nil {
					return err
				}
			}
			return nil
		})
	})
}
