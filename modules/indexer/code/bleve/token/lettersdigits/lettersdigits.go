// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// Package lettersdigits provides a tokenizer for repoIndexerAnalyzer (code
// content search). It groups runs of letters and digits into one token
// (so "console.log" still splits into "console"/"log" on the period, and
// "file3"/"699" tokenize as themselves instead of losing their digits — see
// #37221), while emitting CJK ideograph/kana/hangul runes as individual
// single-character tokens instead of gluing a whole CJK phrase into one
// unsearchable token, matching the per-character segmentation bleve's
// `unicode` tokenizer already gives filenameIndexerAnalyzer.
package lettersdigits

import (
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2/analysis"
	"github.com/blevesearch/bleve/v2/registry"
)

const Name = "gitea/lettersdigits"

type Tokenizer struct{}

func NewTokenizer() *Tokenizer {
	return &Tokenizer{}
}

func TokenizerConstructor(config map[string]any, cache *registry.Cache) (analysis.Tokenizer, error) {
	return NewTokenizer(), nil
}

// isCJK reports whether r belongs to a script that is conventionally
// segmented character-by-character rather than by whitespace/punctuation
// word boundaries.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

func (t *Tokenizer) Tokenize(input []byte) analysis.TokenStream {
	rv := make(analysis.TokenStream, 0, 1024)
	position := 0

	runStart, runEnd := 0, 0
	flushRun := func() {
		if runEnd > runStart {
			position++
			rv = append(rv, &analysis.Token{
				Term:     input[runStart:runEnd],
				Start:    runStart,
				End:      runEnd,
				Position: position,
				Type:     analysis.AlphaNumeric,
			})
		}
	}

	offset := 0
	for offset < len(input) {
		r, size := utf8.DecodeRune(input[offset:])
		switch {
		case isCJK(r):
			flushRun()
			position++
			rv = append(rv, &analysis.Token{
				Term:     input[offset : offset+size],
				Start:    offset,
				End:      offset + size,
				Position: position,
				Type:     analysis.Ideographic,
			})
			runStart = offset + size
			runEnd = runStart
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if runEnd != offset {
				// non-contiguous (a CJK/other token was just flushed at this
				// offset already) — start a fresh run here.
				runStart = offset
			}
			runEnd = offset + size
		default:
			flushRun()
			runStart = offset + size
			runEnd = runStart
		}
		offset += size
	}
	flushRun()

	return rv
}

func init() {
	// FIXME: move it to the bleve's init function, but do not call it in global init
	err := registry.RegisterTokenizer(Name, TokenizerConstructor)
	if err != nil {
		panic(err)
	}
}
