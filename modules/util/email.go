// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"errors"
	"strings"

	"gitea.dev/modules/container"

	"golang.org/x/net/idna"
)

var emailDomainProfile = idna.New(idna.MapForLookup(), idna.BidiRule(), idna.VerifyDNSLength(true))

// EmailDomainToASCII returns the punycode form of an email domain, rejecting spellings that differ from it by more than case
func EmailDomainToASCII(domain string) (string, error) {
	asciiDomain, err := emailDomainProfile.ToASCII(domain)
	if err != nil {
		return "", err
	}
	unicodeDomain, err := emailDomainProfile.ToUnicode(asciiDomain)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(domain, asciiDomain) && !strings.EqualFold(domain, unicodeDomain) {
		return "", errors.New("email domain differs from its IDNA form by more than case")
	}
	return asciiDomain, nil
}

// EmailToASCII returns email with its domain in punycode, or unchanged when the domain isn't valid IDNA
func EmailToASCII(email string) string {
	localPart, domain, found := strings.CutLast(email, "@")
	if !found {
		return email
	}
	asciiDomain, err := EmailDomainToASCII(domain)
	if err != nil {
		return email
	}
	return localPart + "@" + asciiDomain
}

// ToLowerEmail returns the case-insensitive identity of an email address, the same for both spellings of an IDN domain
func ToLowerEmail(email string) string {
	return strings.ToLower(EmailToASCII(email))
}

// LowerEmailSpellings returns every lowercase spelling of an email that a stored lower_email may hold
func LowerEmailSpellings(email string) []string {
	localPart, domain, _ := strings.CutLast(email, "@")
	asciiDomain, err := EmailDomainToASCII(domain)
	if err != nil {
		return []string{strings.ToLower(email)}
	}
	unicodeDomain, _ := emailDomainProfile.ToUnicode(asciiDomain)
	return container.SetOf(strings.ToLower(email), strings.ToLower(localPart+"@"+asciiDomain), strings.ToLower(localPart+"@"+unicodeDomain)).Values()
}

// ToLowerEmailDomain returns the punycode form of an email domain, or its ASCII-lowercased input when that isn't valid IDNA
func ToLowerEmailDomain(domain string) string {
	if asciiDomain, err := EmailDomainToASCII(domain); err == nil {
		return asciiDomain
	}
	return ToLowerASCII(domain)
}
