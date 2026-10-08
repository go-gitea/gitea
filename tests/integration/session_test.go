// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"testing"
	"time"

	"gitea.dev/models/auth"
	"gitea.dev/modules/timeutil"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSession(t *testing.T) {
	defer tests.PrintCurrentTest(t)()
	defer timeutil.MockSet(time.Now())()
	key := "0123456789abcdef"
	for _, create := range []bool{true, true, false} {
		require.NoError(t, auth.UpdateSession(t.Context(), key, []byte("data"), create))
	}
	sess, exist, err := auth.GetSession(t.Context(), key)
	require.NoError(t, err)
	require.True(t, exist)
	assert.Equal(t, []byte("data"), sess.Data)

	require.NoError(t, auth.DestroySession(t.Context(), key))
	require.NoError(t, auth.UpdateSession(t.Context(), key, []byte("data"), false))
	_, exist, err = auth.GetSession(t.Context(), key)
	require.NoError(t, err)
	assert.False(t, exist)
}
