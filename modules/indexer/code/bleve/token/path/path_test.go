// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package path

import (
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/unicode"
	"github.com/stretchr/testify/assert"
)

type Scenario struct {
	Input  string
	Tokens []string
}

func TestTokenFilter(t *testing.T) {
	scenarios := []struct {
		Input string
		Terms []string
	}{
		{
			Input: "Dockerfile",
			Terms: []string{"Dockerfile"},
		},
		{
			Input: "Dockerfile.rootless",
			Terms: []string{"Dockerfile.rootless"},
		},
		{
			Input: "a/b/c/Dockerfile.rootless",
			Terms: []string{"a", "a/b", "a/b/c", "a/b/c/Dockerfile.rootless", "Dockerfile.rootless"},
		},
		{
			Input: "",
			Terms: []string{},
		},
	}

	for _, scenario := range scenarios {
		t.Run(fmt.Sprintf("ensure terms of '%s'", scenario.Input), func(t *testing.T) {
			assert.Equal(t, scenario.Terms, extractTerms(scenario.Input))
		})
	}
}

func TestTokenFilterOffsets(t *testing.T) {
	const input = "potato/ham.md"
	var spans []string
	for _, token := range filter(tokenize(input)) {
		spans = append(spans, string(token.Term)+"="+input[token.Start:token.End])
	}
	assert.Equal(t, []string{"potato=potato", "potato/ham.md=potato/ham.md", "ham.md=ham.md"}, spans)
}

func extractTerms(input string) []string {
	tokens := tokenize(input)
	filteredTokens := filter(tokens)
	terms := make([]string, 0, len(filteredTokens))

	for _, token := range filteredTokens {
		terms = append(terms, string(token.Term))
	}

	return terms
}

func filter(input analysis.TokenStream) analysis.TokenStream {
	filter := NewTokenFilter()
	return filter.Filter(input)
}

func tokenize(input string) analysis.TokenStream {
	tokenizer := unicode.NewUnicodeTokenizer()
	return tokenizer.Tokenize([]byte(input))
}
