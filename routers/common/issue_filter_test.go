// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	"code.gitea.io/gitea/modules/optional"

	"github.com/stretchr/testify/assert"
)

func TestParseIssueFilterStateIsClosed(t *testing.T) {
	assert.Equal(t, optional.None[bool](), ParseIssueFilterStateIsClosed(""))
	assert.Equal(t, optional.None[bool](), ParseIssueFilterStateIsClosed("all"))
	assert.Equal(t, optional.Some(true), ParseIssueFilterStateIsClosed("closed"))
	assert.Equal(t, optional.Some(false), ParseIssueFilterStateIsClosed("open"))
	assert.Equal(t, optional.Some(false), ParseIssueFilterStateIsClosed("unknown"))
}
