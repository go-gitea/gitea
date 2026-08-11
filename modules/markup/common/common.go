// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"regexp"
	"slices"
	"strings"
	"sync"

	"gitea.dev/modules/container"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"mvdan.cc/xurls/v2"
)

type globalVarsType struct {
	schemeRegexp *regexp.Regexp // extract the scheme part from a link

	wwwURLRegexp *regexp.Regexp // matching "www.{any-site}/{any-path}" pattern
	LinkifyRegex *regexp.Regexp // fast matching a URL link (powered by "xurls" package with custom schemes), no any extra validation.

	allowedSchemes    []string // nil means "allow all" (but disable the unsafe ones)
	DisallowedSchemes []string
}

const regexpScheme = `[a-z][-+.a-z0-9]*`

var GlobalVars = sync.OnceValue(func() *globalVarsType {
	v := &globalVarsType{}
	v.schemeRegexp = regexp.MustCompile(`^` + regexpScheme + `:`)
	v.wwwURLRegexp = regexp.MustCompile(`^www\.[-a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}((?:/|[#?])[-a-zA-Z0-9@:%_\+.~#!?&//=\(\);,'">\^{}\[\]` + "`" + `]*)?`)
	v.LinkifyRegex, _ = xurls.StrictMatchingScheme("https?://")
	v.allowedSchemes = []string{"http", "https"}
	v.DisallowedSchemes = []string{"data", "javascript", "vbscript"}
	return v
})

func CheckLinkURLScheme(link string) (abs, valid bool) {
	vars := GlobalVars()
	m := vars.schemeRegexp.FindStringSubmatch(link)
	if m == nil {
		return false, true // relative link is always valid
	}
	urlScheme := m[0]
	urlScheme = urlScheme[0 : len(urlScheme)-1] // remove the trailing ":"
	allowed := len(vars.allowedSchemes) == 0 || slices.Contains(vars.allowedSchemes, urlScheme)
	disabled := slices.Contains(vars.DisallowedSchemes, urlScheme)
	return true, allowed && !disabled
}

func InitLinkURLSchemes(customSchemes []string) {
	validScheme := regexp.MustCompile(`^` + regexpScheme + `$`)
	schemes := container.Set[string]{}
	for _, scheme := range customSchemes {
		schemeLower := strings.ToLower(scheme)
		if !validScheme.MatchString(schemeLower) {
			log.Error("Invalid custom URL scheme %q", scheme)
			continue
		}
		schemes.Add(schemeLower)
	}

	// HINT: CUSTOM-URL-SCHEMES-ALLOW: setting custom means also allow them besides http/https, no custom means "allow all"
	if len(schemes) > 0 {
		schemes.AddMultiple("http", "https")
		linkifyRegexps := make([]string, 0, len(schemes))
		for _, s := range schemes.Values() {
			s += util.Iif(slices.Contains(xurls.SchemesNoAuthority, s), ":", "://")
			linkifyRegexps = append(linkifyRegexps, regexp.QuoteMeta(s))
		}
		GlobalVars().LinkifyRegex, _ = xurls.StrictMatchingScheme(strings.Join(linkifyRegexps, "|"))
		GlobalVars().allowedSchemes = schemes.Values()
	} else {
		GlobalVars().LinkifyRegex, _ = xurls.StrictMatchingScheme("https?://") // only auto-linkify http and https
		GlobalVars().allowedSchemes = nil
	}
}
