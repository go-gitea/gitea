// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"gitea.dev/modules/util"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/token/camelcase"
	"github.com/blevesearch/bleve/v2/registry"
)

const camelCaseKeepWholeName = "camelCaseKeepWhole"

// camelCaseKeepWholeFilter behaves like bleve's built-in "camelCase" token filter,
// it also uses the whole word for a token. For example: when indexing "someThing",
// CamelCaseFilter only emits "some" and "thing", this filter also emits "something".
// It is questionable why the "issue indexer" used the CamelCaseFilter, it just can't search "someThing".
// To avoid breaking existing user experiences, this "whole token filter" is introduced to make the full word can be searched.
type camelCaseKeepWholeFilter struct {
	inner *camelcase.CamelCaseFilter
}

func (f *camelCaseKeepWholeFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	// First, do exactly what the stock camelCase filter does: split by "camelCase" tokens
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

func camelCaseKeepWholeFilterConstructor(_ map[string]any, _ *registry.Cache) (analysis.TokenFilter, error) {
	return &camelCaseKeepWholeFilter{inner: camelcase.NewCamelCaseFilter()}, nil
}

func init() {
	util.MustNoError(registry.RegisterTokenFilter(camelCaseKeepWholeName, camelCaseKeepWholeFilterConstructor))
}
