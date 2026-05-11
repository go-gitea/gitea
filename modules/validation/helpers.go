// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package validation

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/setting"
)

type globalVarsStruct struct {
	externalTrackerRegex    *regexp.Regexp
	validUsernamePattern    *regexp.Regexp
	invalidUsernamePattern  *regexp.Regexp
	validBadgeSlugPattern   *regexp.Regexp
	invalidBadgeSlugPattern *regexp.Regexp
}

var globalVars = sync.OnceValue(func() *globalVarsStruct {
	return &globalVarsStruct{
		externalTrackerRegex:    regexp.MustCompile(`({?)(?:user|repo|index)+?(}?)`),
		validUsernamePattern:    regexp.MustCompile(`^[\da-zA-Z][-.\w]*$`),
		invalidUsernamePattern:  regexp.MustCompile(`[-._]{2,}|[-._]$`), // No consecutive or trailing non-alphanumeric chars
		validBadgeSlugPattern:   regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`),
		invalidBadgeSlugPattern: regexp.MustCompile(`[-._]{2,}|[-._]$`),
	}
})

// IsValidURL checks if URL is valid
func IsValidURL(uri string) bool {
	if u, err := url.ParseRequestURI(uri); err != nil ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		!validPort(portOnly(u.Host)) {
		return false
	}

	return true
}

// IsValidSiteURL checks if URL is valid
func IsValidSiteURL(uri string) bool {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return false
	}

	if !validPort(portOnly(u.Host)) {
		return false
	}

	return slices.Contains(setting.Service.ValidSiteURLSchemes, u.Scheme)
}

// IsEmailDomainListed checks whether the domain of an email address
// matches a list of domains
func IsEmailDomainListed(globs []glob.Glob, email string) bool {
	if len(globs) == 0 {
		return false
	}

	n := strings.LastIndex(email, "@")
	if n <= 0 {
		return false
	}

	domain := strings.ToLower(email[n+1:])

	for _, g := range globs {
		if g.Match(domain) {
			return true
		}
	}

	return false
}

// IsValidExternalTrackerURLFormat checks if URL matches required syntax for external trackers
func IsValidExternalTrackerURLFormat(uri string) bool {
	if !IsValidURL(uri) {
		return false
	}
	vars := globalVars()
	// check for typoed variables like /{index/ or /[repo}
	for _, match := range vars.externalTrackerRegex.FindAllStringSubmatch(uri, -1) {
		if (match[1] == "{" || match[2] == "}") && (match[1] != "{" || match[2] != "}") {
			return false
		}
	}

	return true
}

// IsValidUsername checks if username is valid
func IsValidUsername(name string) bool {
	// It is difficult to find a single pattern that is both readable and effective,
	// but it's easier to use positive and negative checks.
	vars := globalVars()
	return vars.validUsernamePattern.MatchString(name) && !vars.invalidUsernamePattern.MatchString(name)
}

func IsValidBadgeSlug(slug string) bool {
	vars := globalVars()
	return vars.validBadgeSlugPattern.MatchString(slug) && !vars.invalidBadgeSlugPattern.MatchString(slug)
}
