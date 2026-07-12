// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePatchHeadSHA(t *testing.T) {
	patchSHA := "0123456789abcdef0123456789abcdef01234567"
	patchContent := "From " + patchSHA + " Mon Sep 17 00:00:00 2001\n"

	headSHA, err := parsePatchHeadSHA(strings.NewReader(patchContent))
	require.NoError(t, err)
	require.Equal(t, patchSHA, headSHA)
}
