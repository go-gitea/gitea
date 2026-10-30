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
	token := generateToken()
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
			state := pages_model.DomainStatePending
			if !setting.Pages.RequireGiteaTXTRecord {
				state = pages_model.DomainStateActive
				for _, existing := range domains {
					if existing.State == pages_model.DomainStateActive {
						existing.State = pages_model.DomainStateReleasing
						if err := pages_model.UpdateDomain(ctx, existing); err != nil {
							return err
						}
					}
				}
			}
			created = &pages_model.Domain{
				RepoID: repoID, Host: host, TXTToken: token,
				State: state, TXTResult: pages_model.TXTResultPending,
				CreatedUnixNano: updated,
			}
			if err := pages_model.InsertDomain(ctx, created); err != nil {
				return err
			}
			if state == pages_model.DomainStateActive {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpdateSite(ctx, site)
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
			if err := pages_model.UpdateDomain(ctx, domain); err != nil {
				return err
			}
			site, has, err := pages_model.GetSiteByRepoID(ctx, repoID)
			if err != nil {
				return err
			}
			if has {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpdateSite(ctx, site)
			}
			return nil
		})
	})
	if err != nil {
		return fmt.Errorf("RemoveDomain: %w", err)
	}
	return ReleaseConfirmedDomains(ctx, repoID)
}

// VerifyDueDomains verifies a bounded batch using the configured TXT requirement.
func VerifyDueDomains(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 500 {
		return errors.New("VerifyDueDomains: invalid batch size")
	}
	domains, err := listDomainsDueForVerification(ctx, limit)
	if err != nil {
		return fmt.Errorf("VerifyDueDomains: query: %w", err)
	}
	resolver := pagesTXTResolver()
	var allErrors error
	for _, domain := range domains {
		if err := verifyDomain(ctx, domain.ID, resolver.LookupTXT); err != nil {
			log.Warn("Pages TXT check for domain %d: %v", domain.ID, err)
			allErrors = errors.Join(allErrors, fmt.Errorf("VerifyDueDomains: domain %d: %w", domain.ID, err))
		}
	}
	return allErrors
}

func listDomainsDueForVerification(ctx context.Context, limit int) ([]*pages_model.Domain, error) {
	domains := make([]*pages_model.Domain, 0, limit)
	query := db.GetEngine(ctx)
	if setting.Pages.RequireGiteaTXTRecord {
		cutoff := timeutil.TimeStampNow().AddDuration(-setting.Pages.DomainVerificationInterval)
		query = query.Where("state != ? AND txt_last_check_unix <= ?", pages_model.DomainStateReleasing, cutoff).Asc("txt_last_check_unix", "id")
	} else {
		query = query.Where("state = ?", pages_model.DomainStatePending).Asc("id")
	}
	if err := query.Limit(limit).Find(&domains); err != nil {
		return nil, err
	}
	return domains, nil
}

func pagesTXTResolver() *net.Resolver {
	if setting.Pages.DNSResolver != "" {
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		return &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, setting.Pages.DNSResolver)
			},
		}
	}
	return net.DefaultResolver
}

func verifyDomain(ctx context.Context, domainID int64, lookupTXT func(context.Context, string) ([]string, error)) error {
	domain, has, err := pages_model.GetDomainByID(ctx, domainID)
	if err != nil || !has || domain.State == pages_model.DomainStateReleasing {
		return err
	}
	requireTXT := setting.Pages.RequireGiteaTXTRecord
	matched, lookupErr := false, error(nil)
	if requireTXT {
		var records []string
		lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		records, lookupErr = lookupTXT(lookupCtx, "_gitea-pages."+domain.Host)
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
			oldResult := current.TXTResult
			oldEffective := domainPublishingEligible(current, requireTXT, updated)
			wasPending := current.State == pages_model.DomainStatePending
			if requireTXT {
				current.TXTLastCheckUnix = int64(timeutil.TimeStampNow())
			}
			switch {
			case !requireTXT:
			case matched:
				current.TXTResult = pages_model.TXTResultValid
				current.TXTLastSuccessUnixNano = updated
			case lookupErr != nil && !isTXTRecordAbsent(lookupErr):
				current.TXTResult = pages_model.TXTResultTemporaryError
			default:
				current.TXTResult = pages_model.TXTResultInvalid
			}
			if current.State == pages_model.DomainStatePending && (!requireTXT || matched) {
				existing, err := pages_model.ListDomainsByRepoID(ctx, current.RepoID)
				if err != nil {
					return err
				}
				for _, old := range existing {
					if old.ID != current.ID && old.State == pages_model.DomainStateActive {
						old.State = pages_model.DomainStateReleasing
						if err := pages_model.UpdateDomain(ctx, old); err != nil {
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
			if err := pages_model.UpdateDomain(ctx, current); err != nil {
				return err
			}
			if current.State == pages_model.DomainStateActive && (oldResult != current.TXTResult || oldEffective != domainPublishingEligible(current, requireTXT, updated) || wasPending) {
				site.PagesUpdatedUnixNano = updated
				return pages_model.UpdateSite(ctx, site)
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

func domainPublishingEligible(domain *pages_model.Domain, requireTXT bool, now int64) bool {
	if domain.State != pages_model.DomainStateActive {
		return false
	}
	if !requireTXT {
		return true
	}
	if domain.TXTLastSuccessUnixNano == 0 || domain.TXTLastSuccessUnixNano > now {
		return false
	}
	return domain.TXTResult == pages_model.TXTResultValid || now-domain.TXTLastSuccessUnixNano < int64(24*time.Hour)
}
