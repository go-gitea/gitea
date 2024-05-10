// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
package blenderid

import (
	"regexp"
	"strings"

	"code.gitea.io/gitea/models/user"

	"github.com/mozillazg/go-unidecode"
)

var (
	reInvalidCharsPattern = regexp.MustCompile(`[^\da-zA-Z.\w-]+`)

	// Consecutive non-alphanumeric at start:
	reConsPrefix = regexp.MustCompile(`^[._-]+`)
	reConsSuffix = regexp.MustCompile(`[._-]+$`)
	reConsInfix  = regexp.MustCompile(`[._-]{2,}`)
)

// gitealizeUsername turns a valid Blender ID nickname into a valid Gitea username.
func gitealizeUsername(bidNickname string) string {
	// Remove accents and other non-ASCIIness.
	asciiUsername := unidecode.Unidecode(bidNickname)
	asciiUsername = strings.TrimSpace(asciiUsername)
	asciiUsername = strings.ReplaceAll(asciiUsername, " ", "_")

	err := user.IsUsableUsername(asciiUsername)
	if err == nil && len(asciiUsername) <= 40 {
		return asciiUsername
	}

	newUsername := asciiUsername
	newUsername = reInvalidCharsPattern.ReplaceAllString(newUsername, "_")
	newUsername = reConsPrefix.ReplaceAllString(newUsername, "")
	newUsername = reConsSuffix.ReplaceAllString(newUsername, "")
	newUsername = reConsInfix.ReplaceAllStringFunc(
		newUsername,
		func(match string) string {
			firstRune := []rune(match)[0]
			return string(firstRune)
		})

	if newUsername == "" {
		// Everything was stripped and nothing was left. Better to keep as-is and
		// just let Gitea bork on it.
		return asciiUsername
	}

	// This includes a test for reserved names, which are easily circumvented by
	// appending another character.
	if user.IsUsableUsername(newUsername) != nil {
		if len(newUsername) > 39 {
			return newUsername[:39] + "2"
		}
		return newUsername + "2"
	}

	if len(newUsername) > 40 {
		return newUsername[:40]
	}
	return newUsername
}
