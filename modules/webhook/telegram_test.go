// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTelegramIssuesPayload(t *testing.T) {
	p := issueTestPayLoad()

	pl, err := getTelegramIssuesPayload(p)
	require.Nil(t, err)
	require.NotNil(t, pl)

	assert.Equal(t, "[<a href=\"http://localhost:3000/test/repo\">test/repo</a>] Issue closed: <a href=\"http://localhost:3000/test/repo/issues/2\">#2 crash</a> by <a href=\"https://try.gitea.io/user1\">user1</a>\n\n", pl.Message)
}
