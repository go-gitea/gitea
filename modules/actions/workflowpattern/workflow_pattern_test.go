// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package workflowpattern

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchPattern(t *testing.T) {
	kases := []struct {
		inputs       [][]string
		patterns     []string
		skipResult   bool
		filterResult bool
	}{
		{
			patterns:     []string{"*"},
			inputs:       [][]string{{"path/with/slash"}},
			skipResult:   true,
			filterResult: false,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       [][]string{{"meta", "path/b", "otherfile"}},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       [][]string{{"path/b"}, {"path/c", "path/b"}, {"path/c", "path/b", "path/a"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"path/a", "path/b", "path/c"},
			inputs:       [][]string{{"path/c", "path/b", "path/d", "path/a"}},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{},
			inputs:       [][]string{{}},
			skipResult:   false,
			filterResult: false,
		},
		{
			patterns:     []string{"\\!file"},
			inputs:       [][]string{{"!file"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"escape\\\\backslash"},
			inputs:       [][]string{{"escape\\backslash"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{".yml"},
			inputs:       [][]string{{"fyml"}},
			skipResult:   true,
			filterResult: false,
		},
		// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#patterns-to-match-branches-and-tags
		{
			patterns:     []string{"feature/*"},
			inputs:       [][]string{{"feature/my-branch"}, {"feature/your-branch"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"feature/**"},
			inputs:       [][]string{{"feature/beta-a/my-branch"}, {"feature/mona/the/octocat"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"main", "releases/mona-the-octocat"},
			inputs:       [][]string{{"main"}, {"releases/mona-the-octocat"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*"},
			inputs:       [][]string{{"main"}, {"releases"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**"},
			inputs:       [][]string{{"all/the/branches"}, {"every/tag"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*feature"},
			inputs:       [][]string{{"mona-feature"}, {"feature"}, {"ver-10-feature"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v2*"},
			inputs:       [][]string{{"v2"}, {"v2.0"}, {"v2.9"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"v[12].[0-9]+.[0-9]+"},
			inputs:       [][]string{{"v1.10.1"}, {"v2.0.0"}},
			skipResult:   false,
			filterResult: true,
		},
		// https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#patterns-to-match-file-paths
		{
			patterns:     []string{"*"},
			inputs:       [][]string{{"README.md"}, {"server.rb"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.jsx?"},
			inputs:       [][]string{{"page.js"}, {"page.jsx"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**"},
			inputs:       [][]string{{"all/the/files.md"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.js"},
			inputs:       [][]string{{"app.js"}, {"index.js"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**.js"},
			inputs:       [][]string{{"index.js"}, {"js/index.js"}, {"src/js/app.js"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/*"},
			inputs:       [][]string{{"docs/README.md"}, {"docs/file.txt"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**"},
			inputs:       [][]string{{"docs/README.md"}, {"docs/mona/octocat.txt"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"docs/**/*.md"},
			inputs:       [][]string{{"docs/README.md"}, {"docs/mona/hello-world.md"}, {"docs/a/markdown/file.md"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/docs/**"},
			inputs:       [][]string{{"docs/hello.md"}, {"dir/docs/my-file.txt"}, {"space/docs/plan/space.doc"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/README.md"},
			inputs:       [][]string{{"README.md"}, {"js/README.md"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*src/**"},
			inputs:       [][]string{{"a/src/app.js"}, {"my-src/code/js/app.js"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/*-post.md"},
			inputs:       [][]string{{"my-post.md"}, {"path/their-post.md"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"**/migrate-*.sql"},
			inputs:       [][]string{{"migrate-10909.sql"}, {"db/migrate-v1.0.sql"}, {"db/sept/migrate-v1.sql"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md"},
			inputs:       [][]string{{"hello.md"}},
			skipResult:   false,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md"},
			inputs:       [][]string{{"README.md"}, {"docs/hello.md"}},
			skipResult:   true,
			filterResult: true,
		},
		{
			patterns:     []string{"*.md", "!README.md", "README*"},
			inputs:       [][]string{{"hello.md"}, {"README.md"}, {"README.doc"}},
			skipResult:   false,
			filterResult: true,
		},
	}

	for _, kase := range kases {
		for _, inputs := range kase.inputs {
			msg := fmt.Sprintf("patterns=%s, input=%s", strings.Join(kase.patterns, ","), strings.Join(inputs, ","))
			patterns, err := CompilePatterns(kase.patterns...)
			assert.NoError(t, err, "compile error: %s", msg)
			assert.Equal(t, kase.skipResult, Skip(patterns, inputs), "unexpected skip result: %s", msg)
			assert.Equal(t, kase.filterResult, Filter(patterns, inputs), "unexpected filter result: %s", msg)
		}
	}
}
