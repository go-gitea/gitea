// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bleve

import (
	"regexp"
	"unicode"

	"gitea.dev/modules/util"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/analysis/tokenizer/character"
	"github.com/blevesearch/bleve/v2/registry"
)

const codeTokenizerName = "codeTokenizer"

func codeTokenizerConstructor(_ map[string]any, _ *registry.Cache) (analysis.Tokenizer, error) {
	// Old code used "letter" tokenizer which doesn't support CJK.
	// Here it still doesn't support CJK, since there is no usable CJK tokenizer at the moment.
	return character.NewCharacterTokenizer(func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r)
	}), nil
}

const codeTokenFilterName = "codeTokenFilter"

type codeTokenFilter struct {
	re *regexp.Regexp
}

func (c codeTokenFilter) Filter(stream analysis.TokenStream) (ret analysis.TokenStream) {
	// split one token to "letter" parts and "number" parts (to keep the old behavior).
	// e.g.: input token="port123", then the output tokens are "port123", "port", "123"
	for _, token := range stream {
		ret = append(ret, token)
		m := c.re.FindAllIndex(token.Term, -1)
		if len(m) > 1 {
			for _, it := range m {
				p1, p2 := it[0], it[1]
				t := &analysis.Token{
					Start:    token.Start + p1,
					End:      token.Start + p2,
					Term:     token.Term[p1:p2],
					Position: token.Position,
					Type:     analysis.AlphaNumeric,
				}
				ret = append(ret, t)
			}
		}
	}
	return ret
}

func codeTokenFilterConstructor(_ map[string]any, _ *registry.Cache) (analysis.TokenFilter, error) {
	return &codeTokenFilter{
		re: regexp.MustCompile("[a-zA-Z]+|[0-9]+"),
	}, nil
}

func init() {
	util.MustNoError(registry.RegisterTokenizer(codeTokenizerName, codeTokenizerConstructor))
	util.MustNoError(registry.RegisterTokenFilter(codeTokenFilterName, codeTokenFilterConstructor))
}
