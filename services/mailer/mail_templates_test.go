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
		assert.False(t, strings.HasPrefix(name, "mail/base/"), "partial %q must not be listed as a mail", name)
	}
	for _, name := range []string{"mail/base/head", "mail/base/footer", "mail/base/footer_copyright"} {
		assert.True(t, LoadedTemplates().BodyTemplates.HasTemplate(name), "partial %q must stay compiled", name)
	}
}
