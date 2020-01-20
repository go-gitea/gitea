// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2017 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"io"
	"regexp"
	"sync"

	"code.gitea.io/gitea/modules/setting"

	"github.com/microcosm-cc/bluemonday"
)

// Sanitizer is a protection wrapper of *bluemonday.Policy which does not allow
// any modification to the underlying policies once it's been created.
type Sanitizer struct {
	policy *bluemonday.Policy
	init   sync.Once
}

var sanitizer = &Sanitizer{}

// NewSanitizer initializes sanitizer with allowed attributes based on settings.
// Multiple calls to this function will only create one instance of Sanitizer during
// entire application lifecycle.
func NewSanitizer() {
	sanitizer.init.Do(func() {
		ReplaceSanitizer()
	})
}

// ReplaceSanitizer replaces the current sanitizer to account for changes in settings
func ReplaceSanitizer() {
	sanitizer.policy = bluemonday.UGCPolicy()
	// We only want to allow HighlightJS specific classes for code blocks
	sanitizer.policy.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[\w-]+$`)).OnElements("code")

	// Checkboxes
	sanitizer.policy.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	sanitizer.policy.AllowAttrs("checked", "disabled").OnElements("input")

	// Custom URL-Schemes
	sanitizer.policy.AllowURLSchemes(setting.Markdown.CustomURLSchemes...)

	// Allow keyword markup
	sanitizer.policy.AllowAttrs("class").Matching(regexp.MustCompile(`^` + keywordClass + `$`)).OnElements("span")

	// Allow <kbd> tags for keyboard shortcut styling
	sanitizer.policy.AllowElements("kbd")

	// Custom keyword markup
	for _, rule := range setting.ExternalSanitizerRules {
		if rule.Regexp != nil {
			sanitizer.policy.AllowAttrs(rule.AllowAttr).Matching(rule.Regexp).OnElements(rule.Element)
		} else {
			sanitizer.policy.AllowAttrs(rule.AllowAttr).OnElements(rule.Element)
		}
	}
}

// Sanitize takes a string that contains a HTML fragment or document and applies policy whitelist.
func Sanitize(s string) string {
	NewSanitizer()
	return sanitizer.policy.Sanitize(s)
}

// SanitizeReader sanitizes a Reader
func SanitizeReader(r io.Reader) *bytes.Buffer {
	NewSanitizer()
	return sanitizer.policy.SanitizeReader(r)
}

// SanitizeBytes takes a []byte slice that contains a HTML fragment or document and applies policy whitelist.
func SanitizeBytes(b []byte) []byte {
	if len(b) == 0 {
		// nothing to sanitize
		return b
	}
	NewSanitizer()
	return sanitizer.policy.SanitizeBytes(b)
}
