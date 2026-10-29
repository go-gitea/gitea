// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"time"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"
)

var ErrDomainBusy = errors.New("another Pages domain change is still pending")

// AddDomain reserves one custom Host and returns its business TXT value.
func AddDomain(ctx context.Context, repoID int64, rawHost string) (*pages_model.Domain, error) {
	host, err := normalizeDNSHost(rawHost)
	if err != nil || len("_gitea-pages."+host) > 253 {
		return nil, ErrInvalidAddress
	}
	if dnsNamespacesOverlap(host, setting.AppDomain) {
		return nil, ErrDomainConflict
	}
	token, _ := generateTokenAndHash()
	var created *pages_model.Domain
	err = withDomainLock(ctx, func(ctx context.Context) error {
		return withStateChange(ctx, func(ctx context.Context, updated int64) error {
			site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			if !has || site.Deleted {
				return util.ErrNotExist
			}
			if existing, has, err := pages_model.GetDomainByHost(ctx, host); err != nil {
				return err
			} else if has {
				if existing.RepoID == repoID && existing.State != pages_model.DomainStateReleasing {
					created = existing
					return nil
				}
				return ErrDomainConflict
			}
			instances, err := pages_model.ListInstances(ctx)
			if err != nil {
				return err
			}
			for _, instance := range instances {
				if instance.RootURL == "" {
					continue
				}
				root, err := url.Parse(instance.RootURL)
				if err != nil {
					return err
				}
				if dnsNamespacesOverlap(host, root.Hostname()) {
					return ErrDomainConflict
				}
			}
			domains, err := pages_model.ListDomainsByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			for _, existing := range domains {
				if existing.State == pages_model.DomainStatePending || existing.State == pages_model.DomainStateReleasing {
					return ErrDomainBusy
				}
			}
			policyEnabled, _, err := GetTXTVerificationPolicy(ctx)
			if err != nil {
				return err
			}
			state := pages_model.DomainStatePending
			if !policyEnabled {
				state = pages_model.DomainStateActive
				for _, existing := range domains {
					if existing.State == pages_model.DomainStateActive {
						existing.State = pages_model.DomainStateReleasing
						if err := pages_model.UpsertDomain(ctx, existing); err != nil {
							return err
						}
					}
				}
			}
			created = &pages_model.Domain{
				RepoID: repoID, Host: host, TXTToken: token,
				State: state, TXTResult: pages_model.TXTResultPending, TXTNextCheckUnix: int64(timeutil.TimeStampNow()),
				CreatedUnixNano: updated,
			}
			if err := pages_model.UpsertDomain(ctx, created); err != nil {
				return err
			}
			if state == pages_model.DomainStateActive {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpsertSite(ctx, site)
			}
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("AddDomain: %w", err)
	}
	if created.State == pages_model.DomainStateActive {
		if err := ReleaseConfirmedDomains(ctx, repoID); err != nil {
			return nil, fmt.Errorf("AddDomain: release replaced domain: %w", err)
		}
	}
	return created, nil
}

// RemoveDomain starts releasing an active binding or cancels an unverified one.
func RemoveDomain(ctx context.Context, repoID, domainID int64) error {
	err := withDomainLock(ctx, func(ctx context.Context) error {
		return withStateChange(ctx, func(ctx context.Context, updated int64) error {
			domain, has, err := pages_model.GetDomainByID(ctx, domainID)
			if err != nil {
				return err
			}
			if !has || domain.RepoID != repoID {
				return util.ErrNotExist
			}
			if domain.State == pages_model.DomainStatePending {
				return pages_model.DeleteDomainByID(ctx, domainID)
			}
			if domain.State == pages_model.DomainStateReleasing {
				return nil
			}
			domain.State = pages_model.DomainStateReleasing
			if err := pages_model.UpsertDomain(ctx, domain); err != nil {
				return err
			}
			site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			if has {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpsertSite(ctx, site)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("RemoveDomain: %w", err)
	}
	return ReleaseConfirmedDomains(ctx, repoID)
}

// VerifyDueDomains verifies a bounded batch against Gitea's current TXT policy.
func VerifyDueDomains(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 500 {
		return errors.New("VerifyDueDomains: invalid batch size")
	}
	domains := make([]*pages_model.Domain, 0, limit)
	if err := db.GetEngine(ctx).Where("state != ? AND txt_next_check_unix <= ?", pages_model.DomainStateReleasing, timeutil.TimeStampNow()).
		Asc("txt_next_check_unix", "id").Limit(limit).Find(&domains); err != nil {
		return fmt.Errorf("VerifyDueDomains: query: %w", err)
	}
	var allErrors error
	for _, domain := range domains {
		if err := verifyDomain(ctx, domain.ID); err != nil {
			log.Warn("Pages TXT check for domain %d: %v", domain.ID, err)
			allErrors = errors.Join(allErrors, fmt.Errorf("VerifyDueDomains: domain %d: %w", domain.ID, err))
		}
	}
	return allErrors
}

func verifyDomain(ctx context.Context, domainID int64) error {
	domain, has, err := pages_model.GetDomainByID(ctx, domainID)
	if err != nil || !has || domain.State == pages_model.DomainStateReleasing {
		return err
	}
	enabled, _, err := GetTXTVerificationPolicy(ctx)
	if err != nil {
		return err
	}
	matched, lookupErr := false, error(nil)
	if enabled {
		var records []string
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		records, lookupErr = net.DefaultResolver.LookupTXT(lookupCtx, "_gitea-pages."+domain.Host)
		cancel()
		matched = slices.Contains(records, domain.TXTToken)
	}
	var activated bool
	err = withDomainLock(ctx, func(ctx context.Context) error {
		return withStateChange(ctx, func(ctx context.Context, updated int64) error {
			current, has, err := pages_model.GetDomainByID(ctx, domainID)
			if err != nil || !has || current.State == pages_model.DomainStateReleasing {
				return err
			}
			site, has, err := pages_model.GetSiteByRepoID(ctx, current.RepoID)
			if err != nil || !has || site.Deleted {
				return err
			}
			policyNow, policyTime, err := GetTXTVerificationPolicy(ctx)
			if err != nil {
				return err
			}
			if policyNow != enabled {
				current.TXTNextCheckUnix = int64(timeutil.TimeStampNow())
				return pages_model.UpsertDomain(ctx, current)
			}
			oldEffective := domainPublishingEligible(current, enabled, policyTime, updated)
			switch {
			case !enabled || matched:
				current.TXTResult = pages_model.TXTResultValid
				current.TXTLastSuccessUnixNano = updated
			case lookupErr != nil && !isTXTRecordAbsent(lookupErr):
				current.TXTResult = pages_model.TXTResultTemporaryError
			default:
				current.TXTResult = pages_model.TXTResultInvalid
				current.TXTLastSuccessUnixNano = 0
			}
			current.TXTNextCheckUnix = int64(timeutil.TimeStampNow().AddDuration(15 * time.Minute))
			if current.State == pages_model.DomainStatePending && (!enabled || matched) {
				existing, err := pages_model.ListDomainsByRepoID(ctx, current.RepoID)
				if err != nil {
					return err
				}
				for _, old := range existing {
					if old.ID != current.ID && old.State == pages_model.DomainStateActive {
						old.State = pages_model.DomainStateReleasing
						if err := pages_model.UpsertDomain(ctx, old); err != nil {
							return err
						}
					}
				}
				current.State = pages_model.DomainStateActive
				activated = true
			}
			if current.State == pages_model.DomainStatePending && updated-current.CreatedUnixNano >= int64(24*time.Hour) {
				return pages_model.DeleteDomainByID(ctx, current.ID)
			}
			if err := pages_model.UpsertDomain(ctx, current); err != nil {
				return err
			}
			if current.State == pages_model.DomainStateActive && (oldEffective != domainPublishingEligible(current, enabled, policyTime, updated) || domain.State == pages_model.DomainStatePending) {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpsertSite(ctx, site)
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	if activated {
		return ReleaseConfirmedDomains(ctx, domain.RepoID)
	}
	return nil
}

func isTXTRecordAbsent(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func domainPublishingEligible(domain *pages_model.Domain, enabled bool, policyTime, now int64) bool {
	if domain.State != pages_model.DomainStateActive {
		return false
	}
	if !enabled {
		return true
	}
	if domain.TXTLastSuccessUnixNano == 0 || domain.TXTLastSuccessUnixNano < policyTime {
		return false
	}
	if domain.TXTResult == pages_model.TXTResultValid {
		return true
	}
	return domain.TXTResult == pages_model.TXTResultTemporaryError && now-domain.TXTLastSuccessUnixNano < int64(24*time.Hour)
}
