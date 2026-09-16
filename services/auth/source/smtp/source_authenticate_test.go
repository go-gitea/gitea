// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package smtp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDomainAllowed(t *testing.T) {
	assert.True(t, isDomainAllowed("föö.de", "example.com,xn--f-1gaa.de"))
	assert.True(t, isDomainAllowed("xn--f-1gaa.de", "FÖÖ.de"))
	assert.True(t, isDomainAllowed("invalid_domain", "INVALID_DOMAIN"))
	assert.False(t, isDomainAllowed("föö.de", "foo.de"))
	assert.False(t, isDomainAllowed("g\u0130tea.io", "gitea.io"))
}
