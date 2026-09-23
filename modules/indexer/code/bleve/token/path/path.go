// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package path

import (
	"strings"

	"gitea.dev/modules/util"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
)

const Name = "gitea/path"

type TokenFilter struct{}

func NewTokenFilter() *TokenFilter {
	return &TokenFilter{}
}

func TokenFilterConstructor(config map[string]any, cache *registry.Cache) (analysis.TokenFilter, error) {
	return NewTokenFilter(), nil
}

// Filter generates a term for each leading part of the path, like the path hierarchy tokenizer in ES
// (e.g., foo/bar/baz.md generates foo, foo/bar and foo/bar/baz.md), and one for the base name (baz.md)
// to search for filenames without supplying the full path.
func (s *TokenFilter) Filter(input analysis.TokenStream) analysis.TokenStream {
	output := make(analysis.TokenStream, 0, len(input)+1)
	var sb strings.Builder
	for i, token := range input {
		if i > 0 {
			sb.WriteString("/")
		}
		sb.Write(token.Term)
		output = append(output, &analysis.Token{
			Position: 1,
			Start:    input[0].Start,
			End:      token.End,
			Type:     analysis.AlphaNumeric,
			Term:     []byte(sb.String()),
		})
	}
	if len(input) > 1 {
		baseName := input[len(input)-1]
		output = append(output, &analysis.Token{Position: 1, Start: baseName.Start, End: baseName.End, Type: analysis.AlphaNumeric, Term: baseName.Term})
	}
	return output
}

func init() {
	// FIXME: move it to the bleve's init function, but do not call it in global init
	util.MustNoError(registry.RegisterTokenFilter(Name, TokenFilterConstructor))
}
