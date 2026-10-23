// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"github.com/blevesearch/bleve/v2/analysis"
	unicode_tokenizer "github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/blevesearch/bleve/v2/registry"
)

const pathTokenizerName = "pathTokenizer"

// pathTokenizer emits the path suffixes starting at each segment and word, for the prefix query in Search
type pathTokenizer struct{}

func (pathTokenizer) Tokenize(input []byte) (ret analysis.TokenStream) {
	words := unicode_tokenizer.NewUnicodeTokenizer().Tokenize(input)
	for start := range input {
		isWordStart := len(words) > 0 && words[0].Start == start
		if isWordStart {
			words = words[1:]
		}
		if start == 0 || input[start-1] == '/' || isWordStart {
			ret = append(ret, &analysis.Token{
				Start:    start,
				End:      len(input),
				Term:     input[start:],
				Position: len(ret) + 1,
			})
		}
	}
	return ret
}

func pathTokenizerConstructor(_ map[string]any, _ *registry.Cache) (analysis.Tokenizer, error) {
	return pathTokenizer{}, nil
}
