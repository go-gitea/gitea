// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/util"
)

// https://docs.github.com/en/actions/learn-github-actions/variables#naming-conventions-for-configuration-variables
// https://docs.github.com/en/actions/security-guides/encrypted-secrets#naming-your-secrets
var globalVars = sync.OnceValue(func() (ret struct {
	namePattern, forbiddenPrefixPattern *regexp.Regexp
},
) {
	ret.namePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	ret.forbiddenPrefixPattern = regexp.MustCompile("(?i)^(GITEA_|GITHUB_)")
	return ret
})

func ValidateName(name string) error {
	vars := globalVars()
	if !vars.namePattern.MatchString(name) {
		return util.NewInvalidArgumentErrorf("name must start with a letter or underscore and contain only letters, numbers, and underscores")
	}
	if vars.forbiddenPrefixPattern.MatchString(name) {
		return util.NewInvalidArgumentErrorf("name cannot start with 'GITEA_' or 'GITHUB_'")
	}
	if strings.EqualFold(name, "CI") {
		return util.NewInvalidArgumentErrorf("'CI' is a reserved name")
	}
	return nil
}
