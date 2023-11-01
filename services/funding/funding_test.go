// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package funding_test

import (
	"testing"

	funding_service "code.gitea.io/gitea/services/funding"

	"github.com/stretchr/testify/assert"
)

func TestIsFundingConfig(t *testing.T) {
	assert.True(t, funding_service.IsFundingConfig(".gitea/FUNDING.yaml"))
	assert.True(t, funding_service.IsFundingConfig(".gitea/FUNDING.yml"))

	assert.True(t, funding_service.IsFundingConfig(".github/FUNDING.yaml"))
	assert.True(t, funding_service.IsFundingConfig(".github/FUNDING.yml"))

	assert.False(t, funding_service.IsFundingConfig("README.md"))
}
