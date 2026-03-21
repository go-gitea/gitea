// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"regexp"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/util"
)

// https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
// https://docs.github.com/en/actions/security-guides/encrypted-secrets#naming-your-secrets
var globalVars = sync.OnceValue(func() (ret struct {
	namePattern, forbiddenPrefixPattern *regexp.Regexp
},
) {
	ret.namePattern = regexp.MustCompile("(?i)^[A-Z_][A-Z0-9_]*$")
	ret.forbiddenPrefixPattern = regexp.MustCompile("(?i)^GIT(EA|HUB)_")
	return ret
})

func ValidateName(name string) error {
	vars := globalVars()
	if !vars.namePattern.MatchString(name) ||
		vars.forbiddenPrefixPattern.MatchString(name) ||
		strings.EqualFold(name, "CI") /* CI is always set to true in GitHub Actions*/ {
		return util.NewInvalidArgumentErrorf("invalid variable or secret name")
	}
	return nil
}
