// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMailTemplatesBasePartials(t *testing.T) {
	for _, name := range LoadedTemplates().TemplateNames {
		assert.False(t, strings.HasPrefix(name, "base/"), "partial %q must not be listed as a mail", name)
	}
	for _, name := range []string{"base/head", "base/footer", "base/footer_copyright"} {
		assert.True(t, LoadedTemplates().BodyTemplates.HasTemplate(name), "partial %q must stay compiled", name)
	}
}
