// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/system"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"golang.org/x/net/idna"
	"xorm.io/builder"
)

var (
	ErrStateChanged   = errors.New("Pages state changed")
	ErrCursorAhead    = errors.New("Pages synchronization cursor is ahead of the current state")
	ErrInvalidAddress = util.NewInvalidArgumentErrorf("invalid Pages address")
	ErrDomainConflict = util.NewAlreadyExistErrorf("Pages domain conflicts with an existing DNS namespace")
)

var stateClock struct {
	sync.Mutex
	last        int64
	initialized bool
}

func withDomainLock(ctx context.Context, change func(context.Context) error) error {
	return globallock.LockAndDo(ctx, "pages_domain_state", change)
}

func nextStateTime(ctx context.Context) (int64, error) {
	if !stateClock.initialized {
		site := new(pages_model.Site)
		if _, err := db.GetEngine(ctx).Desc("pages_updated_unix_nano").Get(site); err != nil {
			return 0, fmt.Errorf("nextStateTime: read sites: %w", err)
		}
		instance := new(pages_model.Instance)
		if _, err := db.GetEngine(ctx).Desc("root_updated_unix_nano").Get(instance); err != nil {
			return 0, fmt.Errorf("nextStateTime: read instances: %w", err)
		}
		_, settings, err := system.GetAllSettings(ctx)
		if err != nil {
			return 0, fmt.Errorf("nextStateTime: read settings: %w", err)
		}
		policyTime, _ := strconv.ParseInt(settings["pages.txt_verification.updated_unix_nano"], 10, 64)
		stateClock.last = max(site.PagesUpdatedUnixNano, instance.RootUpdatedUnixNano, policyTime)
		stateClock.initialized = true
	}
	now := int64(timeutil.TimeStampNanoNow())
	stateClock.last = max(now, stateClock.last+1)
	return stateClock.last, nil
}

func withStateChange(ctx context.Context, change func(context.Context, int64) error) error {
	stateClock.Lock()
	defer stateClock.Unlock()
	return db.WithTx(ctx, func(ctx context.Context) error {
		updated, err := nextStateTime(ctx)
		if err != nil {
			return err
		}
		return change(ctx, updated)
	})
}

func latestSiteUpdateTime(ctx context.Context) (int64, error) {
	stateClock.Lock()
	defer stateClock.Unlock()
	site := new(pages_model.Site)
	if _, err := db.GetEngine(ctx).Desc("pages_updated_unix_nano").Get(site); err != nil {
		return 0, fmt.Errorf("latestSiteUpdateTime: %w", err)
	}
	return site.PagesUpdatedUnixNano, nil
}

func generateTokenAndHash() (string, string) {
	raw := base64.RawURLEncoding.EncodeToString(util.CryptoRandomBytes(32))
	digest := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(digest[:])
}

// CreateInstance returns the one-time communication credential to the administrator.
func CreateInstance(ctx context.Context) (*pages_model.Instance, string, error) {
	raw, digest := generateTokenAndHash()
	instance := &pages_model.Instance{TokenHash: digest}
	if err := pages_model.UpsertInstance(ctx, instance); err != nil {
		return nil, "", fmt.Errorf("CreateInstance: %w", err)
	}
	return instance, raw, nil
}

// ResetInstanceCredential invalidates the old process and its reported results.
func ResetInstanceCredential(ctx context.Context, instanceID int64) (string, error) {
	raw, digest := generateTokenAndHash()
	err := withDomainLock(ctx, func(ctx context.Context) error {
		return withStateChange(ctx, func(ctx context.Context, updated int64) error {
			instance, has, err := pages_model.GetInstanceByID(ctx, instanceID)
			if err != nil {
				return err
			}
			if !has {
				return util.ErrNotExist
			}
			instance.TokenHash = digest
			instance.RootUpdatedUnixNano = updated
			if err := pages_model.UpsertInstance(ctx, instance); err != nil {
				return err
			}
			_, err = db.GetEngine(ctx).Where("instance_id = ?", instanceID).Delete(new(pages_model.InstanceResult))
			return err
		})
	})
	if err != nil {
		return "", fmt.Errorf("ResetInstanceCredential: %w", err)
	}
	return raw, nil
}

// RemoveInstance ends its role in domain release after the administrator has
// isolated the old process from serving traffic.
func RemoveInstance(ctx context.Context, instanceID int64) error {
	err := withDomainLock(ctx, func(ctx context.Context) error {
		return pages_model.DeleteInstanceByID(ctx, instanceID)
	})
	if err != nil {
		return fmt.Errorf("RemoveInstance: %w", err)
	}
	domains := make([]*pages_model.Domain, 0)
	if err := db.GetEngine(ctx).Where("state = ?", pages_model.DomainStateReleasing).Find(&domains); err != nil {
		return fmt.Errorf("RemoveInstance: releasing domains: %w", err)
	}
	seen := make(map[int64]struct{}, len(domains))
	for _, domain := range domains {
		if _, ok := seen[domain.RepoID]; ok {
			continue
		}
		seen[domain.RepoID] = struct{}{}
		if err := ReleaseConfirmedDomains(ctx, domain.RepoID); err != nil {
			return fmt.Errorf("RemoveInstance: release repo %d: %w", domain.RepoID, err)
		}
	}
	return nil
}

// AuthenticateInstance identifies the caller without storing its raw token.
func AuthenticateInstance(ctx context.Context, authorization string) (*pages_model.Instance, error) {
	raw, found := strings.CutPrefix(authorization, "Bearer ")
	if !found || len(raw) != 43 {
		return nil, util.ErrNotExist
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return nil, util.ErrNotExist
	}
	digest := sha256.Sum256([]byte(raw))
	instance, has, err := db.Get[pages_model.Instance](ctx, builder.Eq{"token_hash": hex.EncodeToString(digest[:])})
	if err != nil {
		return nil, fmt.Errorf("AuthenticateInstance: %w", err)
	}
	if !has {
		return nil, util.ErrNotExist
	}
	return instance, nil
}

func normalizeDNSHost(raw string) (string, error) {
	if raw == "" || strings.HasSuffix(raw, ".") {
		return "", ErrInvalidAddress
	}
	host, err := idna.Lookup.ToASCII(strings.ToLower(raw))
	if err != nil || len(host) > 253 || len(host) < 3 {
		return "", ErrInvalidAddress
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", ErrInvalidAddress
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidAddress
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", ErrInvalidAddress
			}
		}
	}
	return host, nil
}

func normalizeRootURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		u.Port() != "" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", ErrInvalidAddress
	}
	host, err := normalizeDNSHost(u.Hostname())
	if err != nil || u.Host != u.Hostname() {
		return "", "", ErrInvalidAddress
	}
	if len("_acme-challenge."+host) > 253 || len("*."+host) > 253 {
		return "", "", ErrInvalidAddress
	}
	return u.Scheme + "://" + host, host, nil
}

func dnsNamespacesOverlap(a, b string) bool {
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

// RegisterInstanceRootURL accepts the address advertised by the Pages process.
func RegisterInstanceRootURL(ctx context.Context, instanceID int64, tokenHash, raw string) (*pages_model.Instance, error) {
	normalized, host, err := normalizeRootURL(raw)
	if err != nil {
		return nil, err
	}
	if dnsNamespacesOverlap(host, strings.ToLower(setting.AppDomain)) {
		return nil, ErrDomainConflict
	}
	var registered *pages_model.Instance
	err = withDomainLock(ctx, func(ctx context.Context) error {
		return withStateChange(ctx, func(ctx context.Context, updated int64) error {
			instance, has, err := pages_model.GetInstanceByID(ctx, instanceID)
			if err != nil {
				return err
			}
			if !has || !util.CryptoConstTimeEqual(instance.TokenHash, tokenHash) {
				return util.ErrNotExist
			}
			if instance.RootURL == normalized {
				registered = instance
				return nil
			}
			others, err := pages_model.ListInstances(ctx)
			if err != nil {
				return err
			}
			for _, other := range others {
				if other.ID == instanceID || other.RootURL == "" {
					continue
				}
				u, err := url.Parse(other.RootURL)
				if err != nil {
					return err
				}
				if dnsNamespacesOverlap(host, u.Hostname()) {
					return ErrDomainConflict
				}
			}
			domains := make([]*pages_model.Domain, 0)
			if err := db.GetEngine(ctx).Find(&domains); err != nil {
				return err
			}
			for _, domain := range domains {
				if dnsNamespacesOverlap(host, domain.Host) {
					return ErrDomainConflict
				}
			}
			instance.RootURL = normalized
			instance.RootUpdatedUnixNano = updated
			if err := pages_model.UpsertInstance(ctx, instance); err != nil {
				return err
			}
			registered = instance
			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("RegisterInstanceRootURL: %w", err)
	}
	return registered, nil
}

// GetTXTVerificationPolicy returns the current global business-domain verification setting.
func GetTXTVerificationPolicy(ctx context.Context) (bool, int64, error) {
	_, settings, err := system.GetAllSettings(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("GetTXTVerificationPolicy: %w", err)
	}
	enabled := settings["pages.txt_verification.enabled"] != "false"
	updated, err := strconv.ParseInt(settings["pages.txt_verification.updated_unix_nano"], 10, 64)
	if err != nil && settings["pages.txt_verification.updated_unix_nano"] != "" {
		return false, 0, fmt.Errorf("GetTXTVerificationPolicy: invalid saved policy time: %w", err)
	}
	return enabled, updated, nil
}

// SetTXTVerificationPolicy updates the policy once; site rows are unchanged.
func SetTXTVerificationPolicy(ctx context.Context, enabled bool) error {
	return withStateChange(ctx, func(ctx context.Context, updated int64) error {
		current, _, err := GetTXTVerificationPolicy(ctx)
		if err != nil || current == enabled {
			return err
		}
		return system.SetSettings(ctx, map[string]string{
			"pages.txt_verification.enabled":           strconv.FormatBool(enabled),
			"pages.txt_verification.updated_unix_nano": strconv.FormatInt(updated, 10),
		})
	})
}
