// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/registry"
)

// camelCaseKeepWholeName is the name the filter is registered under in
// bleve's analyzer registry, and the value used in the issue indexer's
// custom "token_filters" list.
const camelCaseKeepWholeName = "camelCaseKeepWhole"

// CamelCaseKeepWholeFilter behaves like bleve's built-in "camelCase" token
// filter (it still splits an identifier such as "SomeThing" or "mDNS" into
// its sub-words, e.g. "Some"+"Thing" or "m"+"DNS", so partial/word search
// still works) but it ADDITIONALLY keeps a copy of the original, un-split
// token in the output stream.
//
// Why this is needed:
// The previous analyzer chain was: unicodeNormalize -> camelCase -> to_lower.
// A title like "SomeThing" was indexed only as the two lower-cased terms
// "some" and "thing" - the whole word "something" was never indexed at all.
// Because search uses the very same analyzer chain against the user's query
// text, typing "something" produced the single term "something", which
// never matches the two separate indexed terms "some"/"thing". Typing
// "someThing" (i.e. reproducing the exact original case pattern) still
// produced "some"+"thing" and matched, which made search for camelCase
// identifiers appear to be fully case sensitive except for the first
// character (whose case never affects where camelCase splits a token, so
// it never changed the resulting terms).
//
// By keeping the original token around and letting the following
// "to_lower" filter fold it too, the document ends up indexed with BOTH
// representations ("some", "thing", "something"), and a plain, all
// lower-case query term now matches the whole-word entry regardless of how
// the original text was capitalized.
//
// See: https://github.com/go-gitea/gitea/issues/36228
//
//	https://github.com/go-gitea/gitea/issues/31518
//	https://github.com/go-gitea/gitea/issues/27074
type CamelCaseKeepWholeFilter struct {
	inner *camelcase.CamelCaseFilter
}

// NewCamelCaseKeepWholeFilter creates a new CamelCaseKeepWholeFilter
func NewCamelCaseKeepWholeFilter() *CamelCaseKeepWholeFilter {
	return &CamelCaseKeepWholeFilter{inner: camelcase.NewCamelCaseFilter()}
}

// Filter implements analysis.TokenFilter
func (f *CamelCaseKeepWholeFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	// First, do exactly what the stock camelCase filter does: split every
	// token into its camelCase sub-words. This preserves today's ability
	// to find "mDNS" by searching "DNS", find "SomeThing" by searching
	// "Thing", etc.
	split := f.inner.Filter(input)

	// Index the resulting position of the *first* sub-token produced for
	// each original token (matched by start offset), so the duplicated
	// whole-word token we add below lines up at the same position as the
	// sub-word it stands in for, instead of drifting out of sync for
	// fields with more than one original token.
	posByStart := make(map[int]int, len(split))
	for _, tok := range split {
		if _, ok := posByStart[tok.Start]; !ok {
			posByStart[tok.Start] = tok.Position
		}
	}

	rv := make(analysis.TokenStream, 0, len(split)+len(input))
	rv = append(rv, split...)

	// Then append one extra, un-split copy of every original token, so the
	// whole word survives as a standalone, independently searchable term.
	for _, token := range input {
		dup := *token
		dup.Term = append([]byte(nil), token.Term...)
		if pos, ok := posByStart[token.Start]; ok {
			dup.Position = pos
		}
		rv = append(rv, &dup)
	}

	return rv
}

// CamelCaseKeepWholeFilterConstructor builds a CamelCaseKeepWholeFilter for
// bleve's analyzer registry.
func CamelCaseKeepWholeFilterConstructor(_ map[string]any, _ *registry.Cache) (analysis.TokenFilter, error) {
	return NewCamelCaseKeepWholeFilter(), nil
}

func init() {
	if err := registry.RegisterTokenFilter(camelCaseKeepWholeName, CamelCaseKeepWholeFilterConstructor); err != nil {
		panic(err)
	}
}
