// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, len(gt.Globs()), 3)

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
