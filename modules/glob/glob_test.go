// Copyright 2025 The Gitea Authors. All rights reserved.
// Copyright (c) 2016 Sergey Kamardin
// SPDX-License-Identifier: MIT
//
//nolint:revive // the code is from gobwas/glob
package glob

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reference: https://github.com/gobwas/glob/blob/master/glob_test.go

const (
	pattern_all          = "[a-z][!a-x]*cat*[h][!b]*eyes*"
	regexp_all           = `^[a-z][^a-x].*cat.*[h][^b].*eyes.*$`
	fixture_all_match    = "my cat has very bright eyes"
	fixture_all_mismatch = "my dog has very bright eyes"

	pattern_plain          = "google.com"
	regexp_plain           = `^google\.com$`
	fixture_plain_match    = "google.com"
	fixture_plain_mismatch = "gobwas.com"

	pattern_multiple          = "https://*.google.*"
	regexp_multiple           = `^https:\/\/.*\.google\..*$`
	fixture_multiple_match    = "https://account.google.com"
	fixture_multiple_mismatch = "https://google.com"

	pattern_alternatives          = "{https://*.google.*,*yandex.*,*yahoo.*,*mail.ru}"
	regexp_alternatives           = `^(https:\/\/.*\.google\..*|.*yandex\..*|.*yahoo\..*|.*mail\.ru)$`
	fixture_alternatives_match    = "http://yahoo.com"
	fixture_alternatives_mismatch = "http://google.com"

	pattern_alternatives_suffix                = "{https://*gobwas.com,http://exclude.gobwas.com}"
	regexp_alternatives_suffix                 = `^(https:\/\/.*gobwas\.com|http://exclude.gobwas.com)$`
	fixture_alternatives_suffix_first_match    = "https://safe.gobwas.com"
	fixture_alternatives_suffix_first_mismatch = "http://safe.gobwas.com"
	fixture_alternatives_suffix_second         = "http://exclude.gobwas.com"

	pattern_prefix                 = "abc*"
	regexp_prefix                  = `^abc.*$`
	pattern_suffix                 = "*def"
	regexp_suffix                  = `^.*def$`
	pattern_prefix_suffix          = "ab*ef"
	regexp_prefix_suffix           = `^ab.*ef$`
	fixture_prefix_suffix_match    = "abcdef"
	fixture_prefix_suffix_mismatch = "af"

	pattern_alternatives_combine_lite = "{abc*def,abc?def,abc[zte]def}"
	regexp_alternatives_combine_lite  = `^(abc.*def|abc.def|abc[zte]def)$`
	fixture_alternatives_combine_lite = "abczdef"

	pattern_alternatives_combine_hard = "{abc*[a-c]def,abc?[d-g]def,abc[zte]?def}"
	regexp_alternatives_combine_hard  = `^(abc.*[a-c]def|abc.[d-g]def|abc[zte].def)$`
	fixture_alternatives_combine_hard = "abczqdef"
)

type test struct {
	pattern, match string
	should         bool
	delimiters     []rune
}

func glob(s bool, p, m string, d ...rune) test {
	return test{p, m, s, d}
}

func TestGlob(t *testing.T) {
	for _, test := range []test{
		glob(true, "* ?at * eyes", "my cat has very bright eyes"),

		glob(true, "", ""),
		glob(false, "", "b"),

		glob(true, "*ä", "åä"),
		glob(true, "abc", "abc"),
		glob(true, "a*c", "abc"),
		glob(true, "a*c", "a12345c"),
		glob(true, "a?c", "a1c"),
		glob(true, "a.b", "a.b", '.'),
		glob(true, "a.*", "a.b", '.'),
		glob(true, "a.**", "a.b.c", '.'),
		glob(true, "a.?.c", "a.b.c", '.'),
		glob(true, "a.?.?", "a.b.c", '.'),
		glob(true, "?at", "cat"),
		glob(true, "?at", "fat"),
		glob(true, "*", "abc"),
		glob(true, `\*`, "*"),
		glob(true, "**", "a.b.c", '.'),

		glob(false, "?at", "at"),
		glob(false, "?at", "fat", 'f'),
		glob(false, "a.*", "a.b.c", '.'),
		glob(false, "a.?.c", "a.bb.c", '.'),
		glob(false, "*", "a.b.c", '.'),

		glob(true, "*test", "this is a test"),
		glob(true, "this*", "this is a test"),
		glob(true, "*is *", "this is a test"),
		glob(true, "*is*a*", "this is a test"),
		glob(true, "**test**", "this is a test"),
		glob(true, "**is**a***test*", "this is a test"),

		glob(false, "*is", "this is a test"),
		glob(false, "*no*", "this is a test"),
		glob(true, "[!a]*", "this is a test3"),

		glob(true, "*abc", "abcabc"),
		glob(true, "**abc", "abcabc"),
		glob(true, "???", "abc"),
		glob(true, "?*?", "abc"),
		glob(true, "?*?", "ac"),
		glob(false, "sta", "stagnation"),
		glob(true, "sta*", "stagnation"),
		glob(false, "sta?", "stagnation"),
		glob(false, "sta?n", "stagnation"),

		glob(true, "{abc,def}ghi", "defghi"),
		glob(true, "{abc,abcd}a", "abcda"),
		glob(true, "{a,ab}{bc,f}", "abc"),
		glob(true, "{*,**}{a,b}", "ab"),
		glob(false, "{*,**}{a,b}", "ac"),

		glob(true, "/{rate,[a-z][a-z][a-z]}*", "/rate"),
		glob(true, "/{rate,[0-9][0-9][0-9]}*", "/rate"),
		glob(true, "/{rate,[a-z][a-z][a-z]}*", "/usd"),

		glob(true, "{*.google.*,*.yandex.*}", "www.google.com", '.'),
		glob(true, "{*.google.*,*.yandex.*}", "www.yandex.com", '.'),
		glob(false, "{*.google.*,*.yandex.*}", "yandex.com", '.'),
		glob(false, "{*.google.*,*.yandex.*}", "google.com", '.'),

		glob(true, "{*.google.*,yandex.*}", "www.google.com", '.'),
		glob(true, "{*.google.*,yandex.*}", "yandex.com", '.'),
		glob(false, "{*.google.*,yandex.*}", "www.yandex.com", '.'),
		glob(false, "{*.google.*,yandex.*}", "google.com", '.'),

		glob(true, "*//{,*.}example.com", "https://www.example.com"),
		glob(true, "*//{,*.}example.com", "http://example.com"),
		glob(false, "*//{,*.}example.com", "http://example.com.net"),

		glob(true, pattern_all, fixture_all_match),
		glob(false, pattern_all, fixture_all_mismatch),

		glob(true, pattern_plain, fixture_plain_match),
		glob(false, pattern_plain, fixture_plain_mismatch),

		glob(true, pattern_multiple, fixture_multiple_match),
		glob(false, pattern_multiple, fixture_multiple_mismatch),

		glob(true, pattern_alternatives, fixture_alternatives_match),
		glob(false, pattern_alternatives, fixture_alternatives_mismatch),

		glob(true, pattern_alternatives_suffix, fixture_alternatives_suffix_first_match),
		glob(false, pattern_alternatives_suffix, fixture_alternatives_suffix_first_mismatch),
		glob(true, pattern_alternatives_suffix, fixture_alternatives_suffix_second),

		glob(true, pattern_alternatives_combine_hard, fixture_alternatives_combine_hard),

		glob(true, pattern_alternatives_combine_lite, fixture_alternatives_combine_lite),

		glob(true, pattern_prefix, fixture_prefix_suffix_match),
		glob(false, pattern_prefix, fixture_prefix_suffix_mismatch),

		glob(true, pattern_suffix, fixture_prefix_suffix_match),
		glob(false, pattern_suffix, fixture_prefix_suffix_mismatch),

		glob(true, pattern_prefix_suffix, fixture_prefix_suffix_match),
		glob(false, pattern_prefix_suffix, fixture_prefix_suffix_mismatch),
	} {
		g, err := Compile(test.pattern, test.delimiters...)
		require.NoError(t, err)
		result := g.Match(test.match)
		assert.Equal(t, test.should, result, "pattern %q matching %q should be %v but got %v, compiled=%s", test.pattern, test.match, test.should, result, g.(*globCompiler).regexpPattern)
	}
}

func TestQuoteMeta(t *testing.T) {
	for id, test := range []struct {
		in, out string
	}{
		{
			in:  `[foo*]`,
			out: `\[foo\*\]`,
		},
		{
			in:  `{foo*}`,
			out: `\{foo\*\}`,
		},
		{
			in:  `*?\[]{}`,
			out: `\*\?\\\[\]\{\}`,
		},
		{
			in:  `some text and *?\[]{}`,
			out: `some text and \*\?\\\[\]\{\}`,
		},
	} {
		act := QuoteMeta(test.in)
		assert.Equal(t, test.out, act, "QuoteMeta(%q)", test.in)
		_, err := Compile(act)
		assert.NoError(t, err, "#%d _, err := Compile(QuoteMeta(%q) = %q); err = %q", id, test.in, act, err)
	}
}
