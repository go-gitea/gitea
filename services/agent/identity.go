// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"code.gitea.io/gitea/modules/validation"
)

var (
	reAgentInvalidChars = regexp.MustCompile(`[^0-9A-Za-z._-]+`)
	reAgentDupSep       = regexp.MustCompile(`[-._]{2,}`)
)

// NormalizeEnrollmentUsername converts machine identity input (for example "whoami@host")
// into a valid Gitea username while preserving enough entropy for uniqueness.
func NormalizeEnrollmentUsername(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("username must not be empty")
	}

	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "@", "-")
	s = reAgentInvalidChars.ReplaceAllString(s, "-")
	s = reAgentDupSep.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		return "", errors.New("username contains no usable characters")
	}

	if !unicode.IsLetter(rune(s[0])) && !unicode.IsDigit(rune(s[0])) {
		s = "a" + s
	}

	if len(s) > 40 {
		s = strings.Trim(s[:40], "-._")
	}
	if s == "" {
		s = "agent"
	}

	if !validation.IsValidUsername(s) {
		return "", errors.New("unable to normalize username to a valid value")
	}
	return s, nil
}
