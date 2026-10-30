// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"context"
	"math/big"
	"net/url"
	"strings"

	pages_model "gitea.dev/models/pages"
	repo_model "gitea.dev/models/repo"

	"golang.org/x/net/idna"
)

const (
	ownerAlphabet       = "-._0123456789abcdefghijklmnopqrstuvwxyz"
	ownerDirectAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz-"
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
