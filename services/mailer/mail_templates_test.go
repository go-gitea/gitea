// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mailer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMailBasePartialsCompiled(t *testing.T) {
	for _, name := range []string{"mail/base/head", "mail/base/footer", "mail/base/footer_copyright"} {
		assert.True(t, LoadedTemplates().BodyTemplates.HasTemplate(name), "partial %q must stay compiled", name)
	}
}
