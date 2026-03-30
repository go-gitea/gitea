//go:build highlight_visual_parity

// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"fmt"
	stdhtml "html"
	"io"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/net/html"
)

type visualParityResult struct {
	name          string
	mismatchRate  float64
	mismatchCount int
	comparedCount int
}

func canonicalClass(class string) string {
	class = strings.TrimSpace(class)
	switch {
	case class == "", class == "added-code", class == "removed-code":
		return ""
	case class == "py":
		return "name"
	case strings.HasPrefix(class, "k"):
		return "keyword"
	case strings.HasPrefix(class, "c"):
		return "comment"
	case strings.HasPrefix(class, "s"):
		return "string"
	case strings.HasPrefix(class, "m"):
		return "number"
	case strings.HasPrefix(class, "o"):
		return "operator"
	case strings.HasPrefix(class, "p"):
		return "punct"
	case class == "err":
		return "error"
	default:
		return "name"
	}
}

func htmlClassTrack(rendered string) ([]rune, []string, error) {
	var text []rune
	var classes []string
	var classStack []string

	z := html.NewTokenizer(strings.NewReader(rendered))
	for {
		switch z.Next() {
		case html.ErrorToken:
			if err := z.Err(); err != nil && err != io.EOF {
				return nil, nil, err
			}
			return text, classes, nil
		case html.StartTagToken:
			tag, hasAttr := z.TagName()
			if string(tag) != "span" {
				continue
			}
			class := ""
			for hasAttr {
				k, v, more := z.TagAttr()
				if string(k) == "class" {
					class = string(v)
				}
				hasAttr = more
			}
			classStack = append(classStack, canonicalClass(class))
		case html.EndTagToken:
			tag, _ := z.TagName()
			if string(tag) == "span" && len(classStack) > 0 {
				classStack = classStack[:len(classStack)-1]
			}
		case html.TextToken:
			raw := stdhtml.UnescapeString(string(z.Text()))
			class := ""
			if len(classStack) > 0 {
				class = classStack[len(classStack)-1]
			}
			for _, r := range raw {
				text = append(text, r)
				classes = append(classes, class)
			}
		}
	}
}

func compareRenderedParity(treeSitterHTML, chromaHTML string) (visualParityResult, error) {
	tsText, tsClass, err := htmlClassTrack(treeSitterHTML)
	if err != nil {
		return visualParityResult{}, err
	}
	chText, chClass, err := htmlClassTrack(chromaHTML)
	if err != nil {
		return visualParityResult{}, err
	}

	trimTrailingNewline := func(text []rune, classes []string) ([]rune, []string) {
		if len(text) > 0 && text[len(text)-1] == '\n' {
			return text[:len(text)-1], classes[:len(classes)-1]
		}
		return text, classes
	}
	tsText, tsClass = trimTrailingNewline(tsText, tsClass)
	chText, chClass = trimTrailingNewline(chText, chClass)

	if len(tsText) != len(chText) {
		return visualParityResult{}, fmt.Errorf("text length mismatch: ts=%d chroma=%d", len(tsText), len(chText))
	}
	for i := range tsText {
		if tsText[i] != chText[i] {
			return visualParityResult{}, fmt.Errorf("text mismatch at rune %d: ts=%q chroma=%q", i, string(tsText[i]), string(chText[i]))
		}
	}

	compared := 0
	mismatch := 0
	for i := range tsText {
		r := tsText[i]
		if unicode.IsSpace(r) {
			continue
		}
		if tsClass[i] == "" && chClass[i] == "" {
			continue
		}
		compared++
		if tsClass[i] != chClass[i] {
			mismatch++
		}
	}

	rate := 0.0
	if compared > 0 {
		rate = float64(mismatch) / float64(compared)
	}
	return visualParityResult{
		mismatchRate:  rate,
		mismatchCount: mismatch,
		comparedCount: compared,
	}, nil
}

func TestTreeSitterVisualParityTop50(t *testing.T) {
	samples := visualParityStableTreeSitterSamples()
	executed := 0
	skipped := 0
	totalCompared := 0
	totalMismatch := 0
	maxRate := 0.0
	highMismatchCases := 0
	highMismatchThreshold := 0.30

	for _, tc := range samples {
		t.Run(tc.name, func(t *testing.T) {
			attempt := tryRenderCodeByTreeSitterDetailed(tc.fileName, "", []byte(tc.code), true)
			if !attempt.ok {
				skipped++
				t.Skip("tree-sitter renderer unavailable")
			}
			tsRendered := attempt.rendered

			lexer := DetectChromaLexerByFileName(tc.fileName, "")
			if lexer == nil || lexer.Config().Name == "fallback" {
				skipped++
				t.Skip("chroma lexer unavailable")
			}
			chromaRendered := renderCodeByChromaLexer(lexer, tc.code)

			result, err := compareRenderedParity(string(tsRendered), string(chromaRendered))
			if err != nil {
				t.Fatalf("parity compare failed: %v", err)
			}
			result.name = tc.name
			executed++
			totalCompared += result.comparedCount
			totalMismatch += result.mismatchCount
			if result.mismatchRate > maxRate {
				maxRate = result.mismatchRate
			}
			if result.mismatchRate > highMismatchThreshold {
				highMismatchCases++
			}
			t.Logf("mismatch_rate=%.2f%% mismatches=%d/%d", result.mismatchRate*100, result.mismatchCount, result.comparedCount)
		})
	}

	if executed < len(samples) {
		t.Fatalf("insufficient parity coverage: executed=%d skipped=%d", executed, skipped)
	}
	avgRate := 0.0
	if totalCompared > 0 {
		avgRate = float64(totalMismatch) / float64(totalCompared)
	}
	t.Logf("visual parity summary: executed=%d skipped=%d avg_mismatch=%.2f%% max_mismatch=%.2f%% high_mismatch_cases=%d",
		executed, skipped, avgRate*100, maxRate*100, highMismatchCases)

	if avgRate > 0.20 {
		t.Fatalf("average mismatch rate too high: %.2f%%", avgRate*100)
	}
	if highMismatchCases > 8 {
		t.Fatalf("too many high-mismatch cases: %d", highMismatchCases)
	}
}
