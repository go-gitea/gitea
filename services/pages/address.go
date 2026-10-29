// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"math/big"
	"net/url"
	"strings"
)

const ownerAlphabet = "-._0123456789abcdefghijklmnopqrstuvwxyz"

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
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
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
