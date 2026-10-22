// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"strings"
	"testing"
	"time"

	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeOwners(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Run("ParseCodeOwnersLine", testParseCodeOwnersLine)
	t.Run("AbsolutePathPatterns", testCodeOwnerAbsolutePathPatterns)
	t.Run("PatternMatchTimeout", testCodeOwnerPatternMatchTimeout)
}

func testParseCodeOwnersLine(t *testing.T) {
	type CodeOwnerTest struct {
		Line   string
		Tokens []string
	}

	given := []CodeOwnerTest{
		{Line: "", Tokens: nil},
		{Line: "# comment", Tokens: []string{}},
		{Line: "!.* @user1 @org1/team1", Tokens: []string{"!.*", "@user1", "@org1/team1"}},
		{Line: `.*\\.js @user2 #comment`, Tokens: []string{`.*\.js`, "@user2"}},
		{Line: `docs/(aws|google|azure)/[^/]*\\.(md|txt) @org3 @org2/team2`, Tokens: []string{`docs/(aws|google|azure)/[^/]*\.(md|txt)`, "@org3", "@org2/team2"}},
		{Line: `\#path @org3`, Tokens: []string{`#path`, "@org3"}},
		{Line: `path\ with\ spaces/ @org3`, Tokens: []string{`path with spaces/`, "@org3"}},
		{Line: `/docs/.*\\.md @user1`, Tokens: []string{`/docs/.*\.md`, "@user1"}},
		{Line: `!/assets/.*\\.(bin|exe|msi) @user1`, Tokens: []string{`!/assets/.*\.(bin|exe|msi)`, "@user1"}},
	}

	for _, g := range given {
		tokens := TokenizeCodeOwnersLine(g.Line)
		assert.Equal(t, g.Tokens, tokens, "Codeowners tokenizer failed")
	}
}

func testCodeOwnerAbsolutePathPatterns(t *testing.T) {
	type testCase struct {
		content  string
		file     string
		expected bool
	}

	cases := []testCase{
		{content: "/README.md @user5\n", file: "README.md", expected: true},
		{content: "/docs/.* @user5\n", file: "docs/foo.md", expected: true},
		{content: "/docs/.* @user5\n", file: "other/docs/foo.md", expected: false},
		{content: "README.md @user5\n", file: "README.md", expected: true},
		{content: "!/.* @user5\n", file: "README.md", expected: false},
	}

	for _, c := range cases {
		rules, _ := GetCodeOwnersFromContent(t.Context(), c.content)
		require.NotEmpty(t, rules)
		rule := rules[0]
		regexpMatched, _ := rule.Rule.MatchString(c.file)
		ruleMatched := regexpMatched == !rule.Negative
		assert.Equal(t, c.expected, ruleMatched, "pattern %q against file %q", c.content, c.file)
	}
}

func testCodeOwnerPatternMatchTimeout(t *testing.T) {
	rules, _ := GetCodeOwnersFromContent(t.Context(), "(a+)+ @user5\n")
	require.Len(t, rules, 1)

	maliciousInput := strings.Repeat("a", 30) + "X"
	start := time.Now()
	_, err := rules[0].Rule.MatchString(maliciousInput)
	elapsed := time.Since(start)

	require.Error(t, err, "expected MatchTimeout error on pathological input")
	assert.Less(t, elapsed, time.Second, "match timeout did not bound regex evaluation; took %s", elapsed)
}
