// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatPatch(t *testing.T) {
	repo := &mockRepository{path: "repo1_bare"}

	rd := &bytes.Buffer{}
	err := GetPatch(t.Context(), repo, "8d92fc95^...8d92fc95", rd)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	patchb, err := io.ReadAll(rd)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	patch := string(patchb)
	assert.Regexp(t, "^From 8d92fc95", patch)
	assert.Contains(t, patch, "Subject: [PATCH] Add file2.txt")
}
