// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"testing"

	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestExtractMetadata(t *testing.T) {
	var meta structs.IssueTemplate
	body, err := ExtractMetadata(contentTest, &meta)
	assert.NoError(t, err)
	assert.Equal(t, body, bodyTest)
	assert.Equal(t, metaTest, meta)
	assert.True(t, meta.Valid())
}

var (
	bodyTest    = "This is the body"
	contentTest = `-----
name: Test
about: "A Test"
title: "Test Title"
labels:
  - bug
  - "test label"
-----
` + bodyTest
	metaTest = structs.IssueTemplate{
		Name:   "Test",
		About:  "A Test",
		Title:  "Test Title",
		Labels: []string{"bug", "test label"},
	}
)
