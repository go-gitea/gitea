// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBelongingScopeAssertValid(t *testing.T) {
	assert.NotPanics(t, BelongingScope{}.AssertValid)
	assert.NotPanics(t, BelongingScope{OwnerID: 1}.AssertValid)
	assert.NotPanics(t, BelongingScope{RepoID: 1}.AssertValid)
	assert.Panics(t, BelongingScope{OwnerID: 1, RepoID: 1}.AssertValid)
}
