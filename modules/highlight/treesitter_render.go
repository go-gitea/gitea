// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	"html/template"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/odvcencio/gotreesitter"
)

const htmlEscapeChars = "&<>\"'"

func (renderer *treeSitterRenderer) render(code []byte, trimTrailingNewline bool) (result template.HTML, ok bool) {
	if renderer == nil || renderer.highlighter == nil {
		return "", false
	}

	defer func() {
		if err := recover(); err != nil {
			result = ""
			ok = false
			log.Warn("panic while rendering with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	defer renderer.mu.Unlock()

	if renderer.cache.matches(code) && renderer.cache.code != "" && renderer.cache.trim == trimTrailingNewline {
		return renderer.cache.code, true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altCode != "" && renderer.cache.altTrim == trimTrailingNewline {
		return renderer.cache.altCode, true
	}
	ranges, rangesOK := renderer.highlightRangesLocked(code)
	if !rangesOK {
		return "", false
	}

	var out strings.Builder
	out.Grow(len(code) + len(ranges)*32)

	last := 0
	for _, hr := range ranges {
		start := hr.start
		end := hr.end

		if start < last {
			start = last
		}
		if start > len(code) {
			break
		}
		if end > len(code) {
			end = len(code)
		}
		if end <= start {
			continue
		}

		if start > last {
			writeEscapedBytes(&out, code[last:start])
		}

		className := hr.class
		if className == "" {
			writeEscapedBytes(&out, code[start:end])
		} else {
			out.WriteString(`<span class="`)
			out.WriteString(className)
			out.WriteString(`">`)
			writeEscapedBytes(&out, code[start:end])
			out.WriteString(`</span>`)
		}
		last = end
	}

	if last < len(code) {
		writeEscapedBytes(&out, code[last:])
	}

	content := out.String()
	if trimTrailingNewline {
		content = strings.TrimSuffix(content, "\n")
	}
	rendered := template.HTML(content)

	renderer.cache.code = rendered
	renderer.cache.trim = trimTrailingNewline

	return rendered, true
}

func (renderer *treeSitterRenderer) renderLines(code []byte) (result []template.HTML, ok bool) {
	if renderer == nil || renderer.highlighter == nil {
		return nil, false
	}

	defer func() {
		if err := recover(); err != nil {
			result = nil
			ok = false
			log.Warn("panic while rendering lines with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	defer renderer.mu.Unlock()

	if renderer.cache.matches(code) && renderer.cache.lines != nil {
		return append([]template.HTML(nil), renderer.cache.lines...), true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altLines != nil {
		return append([]template.HTML(nil), renderer.cache.altLines...), true
	}
	ranges, rangesOK := renderer.highlightRangesLocked(code)
	if !rangesOK {
		return nil, false
	}

	lines := make([]template.HTML, 0, bytes.Count(code, []byte("\n"))+1)
	var line strings.Builder
	line.Grow(len(code) / 8)

	flushLine := func() {
		lines = append(lines, template.HTML(line.String()))
		line.Reset()
	}

	appendSegment := func(seg []byte, className string) {
		for len(seg) > 0 {
			nl := bytes.IndexByte(seg, '\n')
			part := seg
			hasNL := false
			if nl >= 0 {
				part = seg[:nl+1]
				seg = seg[nl+1:]
				hasNL = true
			} else {
				seg = nil
			}
			if len(part) > 0 {
				if className == "" {
					writeEscapedBytes(&line, part)
				} else {
					line.WriteString(`<span class="`)
					line.WriteString(className)
					line.WriteString(`">`)
					writeEscapedBytes(&line, part)
					line.WriteString(`</span>`)
				}
			}
			if hasNL {
				flushLine()
			}
		}
	}

	last := 0
	for _, hr := range ranges {
		start := hr.start
		end := hr.end

		if start < last {
			start = last
		}
		if start > len(code) {
			break
		}
		if end > len(code) {
			end = len(code)
		}
		if end <= start {
			continue
		}

		if start > last {
			appendSegment(code[last:start], "")
		}
		appendSegment(code[start:end], hr.class)
		last = end
	}

	if last < len(code) {
		appendSegment(code[last:], "")
	}
	if line.Len() > 0 {
		flushLine()
	}

	renderer.cache.lines = append([]template.HTML(nil), lines...)

	return lines, true
}

func (renderer *treeSitterRenderer) highlightRangesLocked(code []byte) ([]normalizedHighlightRange, bool) {
	if renderer.cache.matches(code) && renderer.cache.ranges != nil {
		return renderer.cache.ranges, true
	}

	ranges, tree, ok := renderer.highlightIncrementalLocked(code)
	if !ok {
		if tree != nil {
			tree.Release()
		}
		// Invalidate cache on parse failures so callers can fallback to Chroma
		// rather than reusing stale tree-sitter output.
		renderer.cache.setTree(nil)
		renderer.cache.setSource(nil)
		renderer.cache.setRanges(nil)
		renderer.cache.clearDerived()
		renderer.cache.altSource = renderer.cache.altSource[:0]
		renderer.cache.altCode = ""
		renderer.cache.altTrim = false
		renderer.cache.altLines = nil
		return nil, false
	}
	if !renderer.cache.matches(code) {
		renderer.cache.archiveCurrentDerivedToAlt()
	}
	renderer.cache.setSource(code)
	renderer.cache.setTree(tree)
	renderer.cache.setRanges(ranges)
	renderer.cache.clearDerived()
	return renderer.cache.ranges, true
}

func (renderer *treeSitterRenderer) highlightIncrementalLocked(code []byte) ([]normalizedHighlightRange, *gotreesitter.Tree, bool) {
	if renderer == nil || renderer.highlighter == nil {
		return nil, nil, false
	}

	tryCompatibility := func(failedTree *gotreesitter.Tree) ([]normalizedHighlightRange, *gotreesitter.Tree, bool) {
		if compatRanges, ok := renderer.highlightCompatibilityLocked(code); ok {
			if failedTree != nil {
				failedTree.Release()
			}
			return compatRanges, nil, true
		}
		return nil, failedTree, false
	}

	oldTree := renderer.cache.tree
	oldSource := renderer.cache.source
	if oldTree != nil && len(oldSource) > 0 {
		if edit, ok := computeSingleInputEdit(oldSource, code); ok {
			oldTree.Edit(edit)
			ranges, newTree := renderer.highlighter.HighlightIncremental(code, oldTree)
			if !renderer.highlightResultUsable(code, newTree, ranges) {
				return tryCompatibility(newTree)
			}
			return renderer.normalizeResolvedRanges(code, ranges), newTree, true
		}
	}
	ranges, tree := renderer.highlighter.HighlightIncremental(code, nil)
	if !renderer.highlightResultUsable(code, tree, ranges) {
		return tryCompatibility(tree)
	}
	return renderer.normalizeResolvedRanges(code, ranges), tree, true
}

func (renderer *treeSitterRenderer) highlightCompatibilityLocked(code []byte) ([]normalizedHighlightRange, bool) {
	if renderer == nil || renderer.highlighter == nil || len(code) == 0 {
		return nil, false
	}

	switch renderer.languageName {
	case "haskell":
		const prefix = "module Main where\n"
		if bytes.Contains(code, []byte("module ")) {
			return nil, false
		}

		wrapped := make([]byte, 0, len(prefix)+len(code))
		wrapped = append(wrapped, prefix...)
		wrapped = append(wrapped, code...)

		ranges, tree := renderer.highlighter.HighlightIncremental(wrapped, nil)
		if tree == nil {
			return nil, false
		}
		defer tree.Release()

		root := tree.RootNode()
		if root == nil || root.HasError() || len(ranges) == 0 {
			return nil, false
		}

		offset := uint32(len(prefix))
		endOffset := offset + uint32(len(code))
		translated := make([]gotreesitter.HighlightRange, 0, len(ranges))
		for _, hr := range ranges {
			if hr.StartByte < offset || hr.EndByte > endOffset {
				continue
			}
			translated = append(translated, gotreesitter.HighlightRange{
				StartByte:    hr.StartByte - offset,
				EndByte:      hr.EndByte - offset,
				Capture:      hr.Capture,
				PatternIndex: hr.PatternIndex,
			})
		}
		if len(translated) == 0 {
			return nil, false
		}
		return renderer.normalizeResolvedRanges(code, translated), true
	case "nginx":
		return renderer.highlightNginxCompatibilityLocked(code)
	default:
		return nil, false
	}
}

func (renderer *treeSitterRenderer) highlightNginxCompatibilityLocked(code []byte) ([]normalizedHighlightRange, bool) {
	normalized, normToOrig, ok := normalizeNginxCompatibilitySource(code)
	if !ok {
		return nil, false
	}

	ranges, tree := renderer.highlighter.HighlightIncremental(normalized, nil)
	if tree == nil {
		return nil, false
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil || root.HasError() || len(ranges) == 0 {
		return nil, false
	}

	normalizedRanges := renderer.normalizeResolvedRanges(normalized, ranges)
	projected := projectNormalizedRanges(normalizedRanges, normToOrig)
	if len(projected) == 0 {
		return nil, false
	}
	return projected, true
}

type compatibilityToken struct {
	start int
	end   int
}

func normalizeNginxCompatibilitySource(code []byte) ([]byte, []int, bool) {
	tokens := tokenizeCompatibilitySource(code, "{};")
	if len(tokens) == 0 {
		return nil, nil, false
	}

	normalized := make([]byte, 0, len(code)*2)
	normToOrig := make([]int, 0, len(code)*2)
	appendInserted := func(b byte) {
		normalized = append(normalized, b)
		normToOrig = append(normToOrig, -1)
	}
	appendMappedToken := func(tok compatibilityToken) {
		for idx := tok.start; idx < tok.end; idx++ {
			normalized = append(normalized, code[idx])
			normToOrig = append(normToOrig, idx)
		}
	}
	appendLineBreak := func(indent int) {
		appendInserted('\n')
		for i := 0; i < indent*2; i++ {
			appendInserted(' ')
		}
	}

	indent := 0
	atLineStart := true
	needSpace := false
	for idx, tok := range tokens {
		text := code[tok.start:tok.end]
		isSingleByte := len(text) == 1
		switch {
		case isSingleByte && text[0] == '{':
			if !atLineStart && needSpace {
				appendInserted(' ')
			}
			appendMappedToken(tok)
			atLineStart = false
			needSpace = false
			indent++
			if idx+1 < len(tokens) && !bytes.Equal(code[tokens[idx+1].start:tokens[idx+1].end], []byte("}")) {
				appendLineBreak(indent)
				atLineStart = true
			}
		case isSingleByte && text[0] == ';':
			appendMappedToken(tok)
			atLineStart = false
			needSpace = false
			if idx+1 < len(tokens) && !bytes.Equal(code[tokens[idx+1].start:tokens[idx+1].end], []byte("}")) {
				appendLineBreak(indent)
				atLineStart = true
			}
		case isSingleByte && text[0] == '}':
			if indent > 0 {
				indent--
			}
			if !atLineStart {
				appendLineBreak(indent)
			}
			appendMappedToken(tok)
			atLineStart = false
			needSpace = false
			if idx+1 < len(tokens) && !bytes.Equal(code[tokens[idx+1].start:tokens[idx+1].end], []byte("}")) {
				appendLineBreak(indent)
				atLineStart = true
			}
		default:
			if !atLineStart && needSpace {
				appendInserted(' ')
			}
			appendMappedToken(tok)
			atLineStart = false
			needSpace = true
		}
	}

	if len(normalized) == 0 {
		return nil, nil, false
	}
	if len(code) > 0 && code[len(code)-1] == '\n' && normalized[len(normalized)-1] != '\n' {
		appendInserted('\n')
	}
	return normalized, normToOrig, true
}

func tokenizeCompatibilitySource(code []byte, structural string) []compatibilityToken {
	tokens := make([]compatibilityToken, 0, len(code)/2)
	for idx := 0; idx < len(code); {
		switch code[idx] {
		case ' ', '\t', '\r', '\n':
			idx++
		default:
			start := idx
			if strings.IndexByte(structural, code[idx]) >= 0 {
				idx++
				tokens = append(tokens, compatibilityToken{start: start, end: idx})
				continue
			}
			for idx < len(code) {
				if strings.IndexByte(structural, code[idx]) >= 0 {
					break
				}
				switch code[idx] {
				case ' ', '\t', '\r', '\n':
					goto emitToken
				default:
					idx++
				}
			}
		emitToken:
			tokens = append(tokens, compatibilityToken{start: start, end: idx})
		}
	}
	return tokens
}

func projectNormalizedRanges(ranges []normalizedHighlightRange, normToOrig []int) []normalizedHighlightRange {
	out := make([]normalizedHighlightRange, 0, len(ranges))
	for _, hr := range ranges {
		segmentStart := -1
		lastOrig := -1
		for pos := hr.start; pos < hr.end && pos < len(normToOrig); pos++ {
			orig := normToOrig[pos]
			if orig < 0 {
				if segmentStart >= 0 {
					out = appendNormalizedRange(out, segmentStart, lastOrig+1, hr.class)
					segmentStart = -1
				}
				lastOrig = -1
				continue
			}
			if segmentStart < 0 {
				segmentStart = orig
				lastOrig = orig
				continue
			}
			if orig == lastOrig+1 {
				lastOrig = orig
				continue
			}
			out = appendNormalizedRange(out, segmentStart, lastOrig+1, hr.class)
			segmentStart = orig
			lastOrig = orig
		}
		if segmentStart >= 0 {
			out = appendNormalizedRange(out, segmentStart, lastOrig+1, hr.class)
		}
	}
	return out
}

func (renderer *treeSitterRenderer) highlightResultUsable(code []byte, tree *gotreesitter.Tree, ranges []gotreesitter.HighlightRange) bool {
	if len(code) == 0 {
		return true
	}
	if tree == nil {
		return false
	}
	root := tree.RootNode()
	if root == nil || root.HasError() {
		return false
	}
	return len(ranges) > 0
}

func appendNormalizedRange(out []normalizedHighlightRange, start, end int, className string) []normalizedHighlightRange {
	if end <= start {
		return out
	}
	n := len(out)
	if n > 0 && out[n-1].class == className && out[n-1].end == start {
		out[n-1].end = end
		return out
	}
	return append(out, normalizedHighlightRange{start: start, end: end, class: className})
}

func (renderer *treeSitterRenderer) normalizeResolvedRanges(code []byte, ranges []gotreesitter.HighlightRange) []normalizedHighlightRange {
	codeLen := len(code)
	out := make([]normalizedHighlightRange, 0, len(ranges))
	last := 0
	for _, hr := range ranges {
		start := int(hr.StartByte)
		end := int(hr.EndByte)
		if start < last {
			start = last
		}
		if start > codeLen {
			break
		}
		if end > codeLen {
			end = codeLen
		}
		if end <= start {
			continue
		}

		className := renderer.captureClass(hr.Capture)
		segment := code[start:end]
		className = renderer.adjustSegmentClass(className, segment)
		if renderer.displayName == "PHP" && className == "nt" {
			switch {
			case bytes.HasPrefix(segment, []byte("<?")):
				out = appendNormalizedRange(out, start, start+2, "o")
				if end > start+2 {
					out = appendNormalizedRange(out, start+2, end, "nx")
				}
				last = end
				continue
			case bytes.Equal(segment, []byte("?>")):
				out = appendNormalizedRange(out, start, end, "o")
				last = end
				continue
			}
		}
		out = appendNormalizedRange(out, start, end, className)
		last = end
	}
	return out
}

func (renderer *treeSitterRenderer) captureClass(capture string) string {
	if renderer.captureClassCache == nil {
		renderer.captureClassCache = make(map[string]string, 32)
	}
	if className, ok := renderer.captureClassCache[capture]; ok {
		return className
	}
	className := treeSitterCaptureToChromaClass(capture)
	renderer.captureClassCache[capture] = className
	return className
}

var swiftBuiltinTypeNames = map[string]struct{}{
	"Any":       {},
	"AnyObject": {},
	"Bool":      {},
	"Character": {},
	"Double":    {},
	"Float":     {},
	"Int":       {},
	"Int8":      {},
	"Int16":     {},
	"Int32":     {},
	"Int64":     {},
	"Never":     {},
	"String":    {},
	"UInt":      {},
	"UInt8":     {},
	"UInt16":    {},
	"UInt32":    {},
	"UInt64":    {},
	"Void":      {},
}

func (renderer *treeSitterRenderer) adjustSegmentClass(className string, segment []byte) string {
	if renderer == nil || renderer.languageName != "swift" {
		return className
	}
	switch {
	case className == "o" && bytes.Equal(segment, []byte("->")):
		return "p"
	case className == "nv" && bytes.Equal(segment, []byte("_")):
		return "kc"
	case className == "kt":
		if _, ok := swiftBuiltinTypeNames[string(segment)]; ok {
			return "nb"
		}
	}
	return className
}

func writeEscapedBytes(out *strings.Builder, src []byte) {
	if len(src) == 0 {
		return
	}
	if !bytes.ContainsAny(src, htmlEscapeChars) {
		_, _ = out.Write(src)
		return
	}
	template.HTMLEscape(out, src)
}
