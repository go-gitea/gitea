// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"strings"
	"testing"

	"gitea.dev/modules/validation"

	"github.com/stretchr/testify/assert"
)

func TestCreateRepoFormGitignoresSize(t *testing.T) {
	form := &CreateRepoForm{RepoName: "repo", Gitignores: strings.Repeat("a", 256)}
	assert.NotEmpty(t, validation.Binder().Validate(t.Context(), form))
}
