// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"bytes"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
)

const codeTokenizerName = "codeTokenizer"

// codeTokenizer emits every distinct lowercase gram of 1 to 3 runes once, search verifies longer terms against the content
type codeTokenizer struct{}

func (codeTokenizer) Tokenize(input []byte) analysis.TokenStream {
	lower := bytes.ToLower(input)
	seen := make(map[string]bool)
	var tokens analysis.TokenStream
	var starts [3]int
	for end, count := 0, 0; end < len(lower); {
		starts[0], starts[1], starts[2] = starts[1], starts[2], end
		count = min(count+1, len(starts))
		_, size := utf8.DecodeRune(lower[end:])
		end += size
		for _, start := range starts[len(starts)-count:] {
			if gram := lower[start:end]; !seen[string(gram)] {
				seen[string(gram)] = true
				tokens = append(tokens, &analysis.Token{Start: start, End: end, Term: gram, Position: len(tokens) + 1, Type: analysis.AlphaNumeric})
			}
		}
	}
	return tokens
}

func codeTokenizerConstructor(_config map[string]any, _cache *registry.Cache) (analysis.Tokenizer, error) {
	return codeTokenizer{}, nil
}
