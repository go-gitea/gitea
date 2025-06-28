// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var giteaTemplate = []byte(`
# Header

# All .go files
**.go

# All text files in /text/
text/*.txt

# All files in modules folders
**/modules/*
`)

func TestGiteaTemplate(t *testing.T) {
	gt := GiteaTemplate{Content: giteaTemplate}
	assert.Len(t, gt.Globs(), 3)

	tt := []struct {
		Path  string
		Match bool
	}{
		{Path: "main.go", Match: true},
		{Path: "a/b/c/d/e.go", Match: true},
		{Path: "main.txt", Match: false},
		{Path: "a/b.txt", Match: false},
		{Path: "text/a.txt", Match: true},
		{Path: "text/b.txt", Match: true},
		{Path: "text/c.json", Match: false},
		{Path: "a/b/c/modules/README.md", Match: true},
		{Path: "a/b/c/modules/d/README.md", Match: false},
	}

	for _, tc := range tt {
		t.Run(tc.Path, func(t *testing.T) {
			match := false
			for _, g := range gt.Globs() {
				if g.Match(tc.Path) {
					match = true
					break
				}
			}
			assert.Equal(t, tc.Match, match)
		})
	}
}

func TestFileNameSanitize(t *testing.T) {
	assert.Equal(t, "test_CON", fileNameSanitize("test_CON"))
	assert.Equal(t, "test CON", fileNameSanitize("test CON "))
	assert.Equal(t, "__traverse__", fileNameSanitize("../traverse/.."))
	assert.Equal(t, "http___localhost_3003_user_test.git", fileNameSanitize("http://localhost:3003/user/test.git"))
	assert.Equal(t, "_", fileNameSanitize("CON"))
	assert.Equal(t, "_", fileNameSanitize("con"))
	assert.Equal(t, "_", fileNameSanitize("\u0000"))
	assert.Equal(t, "目标", fileNameSanitize("目标"))
}

func TestTransformers(t *testing.T) {
	cases := []struct {
		name     string
		expected string
	}{
		{"SNAKE", "abc_def_xyz"},
		{"KEBAB", "abc-def-xyz"},
		{"CAMEL", "abcDefXyz"},
		{"PASCAL", "AbcDefXyz"},
		{"LOWER", "abc_def-xyz"},
		{"UPPER", "ABC_DEF-XYZ"},
		{"TITLE", "Abc_def-Xyz"},
	}

	input := "Abc_Def-XYZ"
	assert.Len(t, defaultTransformers, len(cases))
	for i, c := range cases {
		tf := defaultTransformers[i]
		require.Equal(t, c.name, tf.Name)
		assert.Equal(t, c.expected, tf.Transform(input), "case %s", c.name)
	}
}
