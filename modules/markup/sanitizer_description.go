// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// createRepoDescriptionPolicy returns a minimal more strict policy that is used for
// repository descriptions.
func (st *Sanitizer) createRepoDescriptionPolicy() *bluemonday.Policy {
	policy := bluemonday.NewPolicy()
	policy.AllowStandardURLs()

	// Allow italics and bold.
	policy.AllowElements("i", "b", "em", "strong")

	// Allow code.
	policy.AllowElements("code")

	// Allow links
	policy.AllowAttrs("href", "target", "rel").OnElements("a")

	// Allow classes for emojis
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^emoji$`)).OnElements("img", "span")
	policy.AllowAttrs("aria-label").OnElements("span")

	return policy
}

// SanitizeDescription sanitizes the HTML generated for a repository description.
func SanitizeDescription(s string) string {
	return GetDefaultSanitizer().descriptionPolicy.Sanitize(s)
}
