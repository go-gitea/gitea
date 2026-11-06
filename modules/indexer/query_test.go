// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package indexer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSearchQuery(t *testing.T) {
	for _, test := range []struct {
		name            string
		input           string
		terms           []SearchTerm
		path            string
		language        string
		repos           []string
		owners          []string
		withoutLanguage string
		error           bool
	}{
		{name: "bare syntax", input: `printf("hi") std::string http://x Path:x -foo --force a/b/ /usr/bin /`, terms: []SearchTerm{
			{SearchTermLiteral, `printf("hi")`},
			{SearchTermLiteral, "std::string"},
			{SearchTermLiteral, "http://x"},
			{SearchTermLiteral, "Path:x"},
			{SearchTermLiteral, "-foo"},
			{SearchTermLiteral, "--force"},
			{SearchTermLiteral, "a/b/"},
			{SearchTermLiteral, "/usr/bin"},
			{SearchTermLiteral, "/"},
		}},
		{name: "phrase escapes", input: `"say \"hi\" \\ \q" "/usr/bin/" "AND"`, terms: []SearchTerm{
			{SearchTermLiteral, `say "hi" \ \q`}, {SearchTermLiteral, "/usr/bin/"}, {SearchTermLiteral, "AND"},
		}},
		{name: "unicode whitespace", input: "\tfoo\nbar\u2003baz\r\n", terms: []SearchTerm{
			{SearchTermLiteral, "foo"}, {SearchTermLiteral, "bar"}, {SearchTermLiteral, "baz"},
		}, withoutLanguage: "foo bar baz"},
		{name: "quoted whitespace", input: "\t\"foo  bar\"\u2003baz", terms: []SearchTerm{
			{SearchTermLiteral, "foo  bar"}, {SearchTermLiteral, "baz"},
		}, withoutLanguage: `"foo  bar" baz`},
		{
			name: "qualifiers", input: `repo:alice/one user:alice org:team repo:bob/two repo:alice/one org:alice path:src/./go// language:golang word`,
			terms: []SearchTerm{{SearchTermLiteral, "word"}}, path: "src/go", language: "Go", repos: []string{"alice/one", "bob/two", "alice/one"}, owners: []string{"alice", "team", "alice"},
			withoutLanguage: `repo:alice/one user:alice org:team repo:bob/two repo:alice/one org:alice path:src/./go// word`,
		},
		{
			name: "quoted qualifiers", input: `language:"Go" path:"src/a*[b]?/" repo:"alice/one" user:"alice" org:"team" "two words"`,
			terms: []SearchTerm{{SearchTermLiteral, "two words"}}, path: "src/a*[b]?", language: "Go", repos: []string{"alice/one"}, owners: []string{"alice", "team"},
			withoutLanguage: `path:"src/a*[b]?/" repo:"alice/one" user:"alice" org:"team" "two words"`,
		},
		{name: "remove language among whitespace", input: "  foo\t language:golang \u2003bar  ", terms: []SearchTerm{{SearchTermLiteral, "foo"}, {SearchTermLiteral, "bar"}}, language: "Go", withoutLanguage: "foo bar"},
		{name: "regexps", input: `/usr/bin/ /a\/b/ /\d+\s\w/ /(?i:foo)(?-i:BAR)/ /a\\/ /a\//`, terms: []SearchTerm{
			{SearchTermRegexp, "usr/bin"},
			{SearchTermRegexp, "a/b"},
			{SearchTermRegexp, `\d+\s\w`},
			{SearchTermRegexp, "(?i:foo)(?-i:BAR)"},
			{SearchTermRegexp, `a\\`},
			{SearchTermRegexp, "a/"},
		}},
		{name: "regexp literal syntax", input: `/[(?s)]/ /\Q(?m)\n\E/ /\\n/ /[](?s)]/ /[[:alpha:](?s)]/`, terms: []SearchTerm{
			{SearchTermRegexp, "[(?s)]"}, {SearchTermRegexp, `\Q(?m)\n\E`}, {SearchTermRegexp, `\\n`}, {SearchTermRegexp, "[](?s)]"}, {SearchTermRegexp, "[[:alpha:](?s)]"},
		}},
		{name: "escaped final delimiter is literal", input: `/foo\/`, terms: []SearchTerm{{SearchTermLiteral, `/foo\/`}}},
		{name: "regexp whitespace separates terms", input: `/foo bar/`, terms: []SearchTerm{{SearchTermLiteral, "/foo"}, {SearchTermLiteral, "bar/"}}},
		{name: "term byte limit", input: strings.Repeat("é", 128), terms: []SearchTerm{{SearchTermLiteral, strings.Repeat("é", 128)}}},
		{name: "regexp byte limit", input: "/" + strings.Repeat("a", 256) + "/", terms: []SearchTerm{{SearchTermRegexp, strings.Repeat("a", 256)}}},
		{name: "term count limit", input: strings.Repeat("a ", 15) + "a", terms: func() []SearchTerm {
			terms := make([]SearchTerm, 16)
			for i := range terms {
				terms[i] = SearchTerm{SearchTermLiteral, "a"}
			}
			return terms
		}()},
		{name: "empty", error: true},
		{name: "qualifiers only", input: "language:go repo:owner/repo path:src user:owner", error: true},
		{name: "empty phrase", input: `""`, error: true},
		{name: "unterminated phrase", input: `"foo`, error: true},
		{name: "phrase suffix", input: `"foo"bar`, error: true},
		{name: "empty path", input: "foo path:", error: true},
		{name: "repeated path", input: "foo path:src path:lib", error: true},
		{name: "repeated language", input: "foo language:go language:js", error: true},
		{name: "unknown language", input: "foo language:unknownlanguage", error: true},
		{name: "absolute path", input: "foo path:/src", error: true},
		{name: "root path", input: "foo path:./", error: true},
		{name: "parent segment", input: "foo path:src/../lib", error: true},
		{name: "path star", input: "foo path:src/*", error: true},
		{name: "repo without owner", input: "foo repo:/name", error: true},
		{name: "AND", input: "foo AND bar", error: true},
		{name: "negated path", input: "foo -path:src", error: true},
		{name: "reserved content", input: "foo content:bar", error: true},
		{name: "LF in phrase", input: "\"foo\nbar\"", error: true},
		{name: "NUL in term", input: "foo\x00bar", error: true},
		{name: "too many terms", input: strings.Repeat("a ", 17), error: true},
		{name: "term too long", input: strings.Repeat("é", 129), error: true},
		{name: "regexp too long", input: "/" + strings.Repeat("a", 257) + "/", error: true},
		{name: "empty regexp", input: "//", error: true},
		{name: "invalid regexp", input: "/[/", error: true},
		{name: "regexp line syntax", input: `/\v/ /\z/ /\A/ /\n/ /\r/ /(?s)foo/ /(?m)^foo/ /\Q\\E(?s)./`, terms: []SearchTerm{
			{SearchTermRegexp, `\v`},
			{SearchTermRegexp, `\z`},
			{SearchTermRegexp, `\A`},
			{SearchTermRegexp, `\n`},
			{SearchTermRegexp, `\r`},
			{SearchTermRegexp, "(?s)foo"},
			{SearchTermRegexp, "(?m)^foo"},
			{SearchTermRegexp, `\Q\\E(?s).`},
		}},
		{name: "regexp verb", input: "/(*UTF)foo/", error: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			query, err := ParseSearchQuery(test.input)
			if test.error {
				var queryError *SearchQueryError
				require.ErrorAs(t, err, &queryError)
				assert.NotEmpty(t, queryError.Error())
				assert.Nil(t, query)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.terms, query.Terms)
			assert.Equal(t, test.path, query.Path)
			assert.Equal(t, test.language, query.Language)
			assert.Equal(t, test.repos, query.Repos)
			assert.Equal(t, test.owners, query.Owners)
			withoutLanguage := test.withoutLanguage
			if withoutLanguage == "" {
				withoutLanguage = test.input
			}
			assert.Equal(t, withoutLanguage, query.WithoutLanguage())
		})
	}

	query, err := ParseSearchQuery(`foo /ba+r/ "x y"`)
	require.NoError(t, err)
	assert.Equal(t, []string{"foo", "x y"}, query.Literals())
}
