// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package jobparser

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testdata embed.FS

func ReadTestdata(t *testing.T, name string) []byte {
	content, err := testdata.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return content
}
