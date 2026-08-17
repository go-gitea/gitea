// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"unicode"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/character"
	"github.com/blevesearch/bleve/v2/registry"
)

const codeTokenizerName = "codeTokenizer"

func codeTokenizerConstructor(_ map[string]interface{}, _ *registry.Cache) (analysis.Tokenizer, error) {
	// Old code used "letter" tokenizer which doesn't support CJK.
	// Here it still doesn't support CJK, since there is no usable CJK tokenizer at the moment.
	return character.NewCharacterTokenizer(func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_'
	}), nil
}

func init() {
	err := registry.RegisterTokenizer(codeTokenizerName, codeTokenizerConstructor)
	if err != nil {
		panic(err)
	}
}
