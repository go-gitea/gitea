// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/lexers"
)

func BenchmarkDetectChromaLexerByFileName(b *testing.B) {
	for b.Loop() {
		// BenchmarkDetectChromaLexerByFileName-12    	18214717	        61.35 ns/op
		DetectChromaLexerByFileName("a.sql", "")
	}
}

func BenchmarkDetectChromaLexerWithAnalyze(b *testing.B) {
	code := []byte(strings.Repeat("SELECT * FROM table;\n", 1000))
	b.ResetTimer()
	for b.Loop() {
		// BenchmarkRenderCodeSlowGuess-12    	   87946	     13310 ns/op
		detectChromaLexerWithAnalyze("a", "", code)
	}
}

func BenchmarkChromaAnalyze(b *testing.B) {
	code := strings.Repeat("SELECT * FROM table;\n", 1000)
	b.ResetTimer()
	for b.Loop() {
		// comparing to detectChromaLexerWithAnalyze (go-enry), "chroma/lexers.Analyse" is very slow
		// BenchmarkChromaAnalyze-12    	     519	   2247104 ns/op
		lexers.Analyse(code)
	}
}

func BenchmarkRenderCodeByLexer(b *testing.B) {
	code := strings.Repeat("SELECT * FROM table;\n", 1000)
	lexer := DetectChromaLexerByFileName("a.sql", "")
	b.ResetTimer()
	for b.Loop() {
		// HINT: CODE-HIGHLIGHT-PERFORMANCE: Really slow ....... the regexp2 used by Chroma takes most of the time
		// BenchmarkRenderCodeByLexer-12    	      22	  47159038 ns/op
		RenderCodeByLexer(lexer, code)
	}
}
