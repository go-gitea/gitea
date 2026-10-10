// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestBelongingScopeNormalized(t *testing.T) {
	assert.Equal(t, BelongingScope{OwnerID: 1}, BelongingScope{OwnerID: 1}.Normalized())
	assert.Equal(t, BelongingScope{RepoID: 1}, BelongingScope{RepoID: 1}.Normalized())
	assert.Panics(t, func() { BelongingScope{OwnerID: 1, RepoID: 1}.Normalized() })

	defer test.MockVariableValue(&setting.IsProd, true)()
	defer test.MockVariableValue(&setting.IsInTesting, false)()
	assert.Equal(t, BelongingScope{RepoID: 1}, BelongingScope{OwnerID: 1, RepoID: 1}.Normalized())
}
