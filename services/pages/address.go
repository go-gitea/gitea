// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"math/big"
	"net/netip"
	"net/url"
	"strings"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/setting"

	"golang.org/x/net/idna"
)

const (
	ownerAlphabet       = "-._0123456789abcdefghijklmnopqrstuvwxyz"
	ownerDirectAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz-"
)

var domainNameProfile = idna.New(
	idna.MapForLookup(),
	idna.BidiRule(),
	idna.VerifyDNSLength(true),
)

// DefaultSiteURL derives the public address from the instance root and the
// current repository names. The repository ID remains the internal identity.
func DefaultSiteURL(rootURL, ownerName, repoName string) (string, error) {
	root, err := url.Parse(rootURL)
	if err != nil || root.Host == "" || root.Scheme == "" || ownerName == "" || repoName == "" {
		return "", ErrInvalidAddress
	}
	label, err := encodeOwnerDNSLabel(strings.ToLower(ownerName))
	if err != nil {
		return "", err
	}
	host := label + "." + strings.ToLower(root.Hostname())
	if len(host) > 253 || root.Port() != "" {
		return "", ErrInvalidAddress
	}
	base := root.Scheme + "://" + host + "/"
	if strings.EqualFold(repoName, ".pages") {
		return base, nil
	}
	return base + url.PathEscape(strings.ToLower(repoName)) + "/", nil
}

// FirstDefaultSiteURL returns the first valid default address in instance ID order.
func FirstDefaultSiteURL(ctx context.Context, repo *repo_model.Repository) string {
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		return ""
	}
	for _, instance := range instances {
		if instance.RootURL == "" {
			continue
		}
		address, err := DefaultSiteURL(instance.RootURL, repo.OwnerName, repo.LowerName)
		if err == nil {
			return address
		}
	}
	return ""
}

func encodeOwnerDNSLabel(owner string) (string, error) {
	if owner == "" || len(owner) > 40 {
		return "", ErrInvalidAddress
	}
	direct := len(owner) <= 63 && !strings.HasPrefix(owner, "x-") && owner[0] != '-' && owner[len(owner)-1] != '-'
	for i := 0; i < len(owner); i++ {
		c := owner[i]
		if strings.IndexByte(ownerAlphabet, c) < 0 {
			return "", ErrInvalidAddress
		}
		if strings.IndexByte(ownerDirectAlphabet, c) < 0 {
			direct = false
		}
	}
	if direct {
		return owner, nil
	}
	number := new(big.Int)
	forty := big.NewInt(40)
	for i := 0; i < len(owner); i++ {
		number.Mul(number, forty)
		number.Add(number, big.NewInt(int64(strings.IndexByte(ownerAlphabet, owner[i])+1)))
	}
	return "x-" + number.Text(36), nil
}

func normalizeDomainName(raw string) (string, error) {
	host, err := domainNameProfile.ToASCII(raw)
	if err != nil || !strings.Contains(host, ".") {
		return "", ErrInvalidAddress
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return "", ErrInvalidAddress
	}
	return host, nil
}

func normalizeRootURL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil ||
		u.Port() != "" || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", "", ErrInvalidAddress
	}
	host, err := normalizeDomainName(u.Hostname())
	if err != nil || u.Host != u.Hostname() {
		return "", "", ErrInvalidAddress
	}
	if len("_acme-challenge."+host) > 253 {
		return "", "", ErrInvalidAddress
	}
	return u.Scheme + "://" + host, host, nil
}

func dnsNamespacesOverlap(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	return a == b || strings.HasSuffix(a, "."+b) || strings.HasSuffix(b, "."+a)
}

func checkManagedDNSNamespaceConflict(ctx context.Context, host string, excludedInstanceID int64) error {
	if dnsNamespacesOverlap(host, setting.AppDomain) {
		return ErrDomainConflict
	}
	instances, err := pages_model.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		if instance.ID == excludedInstanceID || instance.RootURL == "" {
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
	return nil
}

func checkInstanceRootConflict(ctx context.Context, host string, excludedInstanceID int64) error {
	if err := checkManagedDNSNamespaceConflict(ctx, host, excludedInstanceID); err != nil {
		return err
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
	return nil
}
