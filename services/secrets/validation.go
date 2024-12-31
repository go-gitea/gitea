// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package secrets

import (
	"regexp"

	"code.gitea.io/gitea/modules/util"
)

// https://docs.github.com/en/actions/security-guides/encrypted-secrets#naming-your-secrets
var (
	namePattern            = regexp.MustCompile("(?i)^[A-Z_][A-Z0-9_]*$")
	forbiddenPrefixPattern = regexp.MustCompile("(?i)^GIT(EA|HUB)_")

	ErrInvalidName = util.NewInvalidArgumentErrorf("invalid secret name")
)

func ValidateName(name string) error {
	if !namePattern.MatchString(name) || forbiddenPrefixPattern.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}
