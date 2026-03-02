// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	"html/template"
	"sort"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/odvcencio/gotreesitter"
)

const htmlEscapeChars = "&<>\"'"

func (renderer *treeSitterRenderer) render(code []byte, trimTrailingNewline bool) (template.HTML, bool) {
	if renderer == nil || renderer.parser == nil || renderer.query == nil || renderer.lang == nil {
		return "", false
	}

	defer func() {
		if err := recover(); err != nil {
			log.Warn("panic while rendering with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	if renderer.cache.matches(code) && renderer.cache.code != "" && renderer.cache.trim == trimTrailingNewline {
		cached := renderer.cache.code
		renderer.mu.Unlock()
		return cached, true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altCode != "" && renderer.cache.altTrim == trimTrailingNewline {
		cached := renderer.cache.altCode
		renderer.mu.Unlock()
		return cached, true
	}
	ranges := renderer.highlightRangesLocked(code)
	renderer.mu.Unlock()

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

	renderer.mu.Lock()
	renderer.cache.code = rendered
	renderer.cache.trim = trimTrailingNewline
	renderer.mu.Unlock()

	return rendered, true
}

func (renderer *treeSitterRenderer) renderLines(code []byte) ([]template.HTML, bool) {
	if renderer == nil || renderer.parser == nil || renderer.query == nil || renderer.lang == nil {
		return nil, false
	}

	defer func() {
		if err := recover(); err != nil {
			log.Warn("panic while rendering lines with gotreesitter: %v\n%s", err, log.Stack(2))
		}
	}()

	renderer.mu.Lock()
	if renderer.cache.matches(code) && renderer.cache.lines != nil {
		out := append([]template.HTML(nil), renderer.cache.lines...)
		renderer.mu.Unlock()
		return out, true
	}
	if renderer.cache.matchesAlt(code) && renderer.cache.altLines != nil {
		out := append([]template.HTML(nil), renderer.cache.altLines...)
		renderer.mu.Unlock()
		return out, true
	}
	ranges := renderer.highlightRangesLocked(code)
	renderer.mu.Unlock()

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

	renderer.mu.Lock()
	renderer.cache.lines = append([]template.HTML(nil), lines...)
	renderer.mu.Unlock()

	return lines, true
}

func (renderer *treeSitterRenderer) highlightRangesLocked(code []byte) []normalizedHighlightRange {
	if renderer.cache.matches(code) && renderer.cache.ranges != nil {
		return renderer.cache.ranges
	}

	ranges, tree := renderer.highlightIncrementalLocked(code)
	if !renderer.cache.matches(code) {
		renderer.cache.archiveCurrentDerivedToAlt()
	}
	renderer.cache.setSource(code)
	renderer.cache.setTree(tree)
	renderer.cache.setRanges(ranges)
	renderer.cache.clearDerived()
	return renderer.cache.ranges
}

func (renderer *treeSitterRenderer) highlightIncrementalLocked(code []byte) ([]normalizedHighlightRange, *gotreesitter.Tree) {
	oldTree := renderer.cache.tree
	oldSource := renderer.cache.source
	if oldTree != nil && len(oldSource) > 0 {
		if edit, ok := computeSingleInputEdit(oldSource, code); ok {
			oldTree.Edit(edit)
			newTree := renderer.parseTree(code, oldTree)
			if oldTree != newTree {
				oldTree.Release()
			}
			if newTree == nil || newTree.RootNode() == nil {
				return nil, newTree
			}
			return renderer.queryNormalizedRanges(newTree), newTree
		}
		oldTree.Release()
	}
	tree := renderer.parseTree(code, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, tree
	}
	return renderer.queryNormalizedRanges(tree), tree
}

func (renderer *treeSitterRenderer) parseTree(code []byte, oldTree *gotreesitter.Tree) *gotreesitter.Tree {
	if renderer == nil || renderer.parser == nil {
		return nil
	}

	var (
		tree *gotreesitter.Tree
		err  error
	)
	if renderer.tokenSourceFactor != nil {
		ts := renderer.tokenSourceFactor(code)
		if oldTree != nil {
			tree, err = renderer.parser.ParseIncrementalWithTokenSource(code, oldTree, ts)
		} else {
			tree, err = renderer.parser.ParseWithTokenSource(code, ts)
		}
	} else if oldTree != nil {
		tree, err = renderer.parser.ParseIncremental(code, oldTree)
	} else {
		tree, err = renderer.parser.Parse(code)
	}
	if err != nil {
		return gotreesitter.NewTree(nil, code, renderer.lang)
	}
	return tree
}

func (renderer *treeSitterRenderer) queryNormalizedRanges(tree *gotreesitter.Tree) []normalizedHighlightRange {
	if renderer == nil || renderer.query == nil || renderer.lang == nil || tree == nil {
		return nil
	}
	root := tree.RootNode()
	if root == nil {
		return nil
	}

	cursor := renderer.query.Exec(root, renderer.lang, tree.Source())
	var ranges []gotreesitter.HighlightRange
	for {
		c, ok := cursor.NextCapture()
		if !ok {
			break
		}
		node := c.Node
		if node == nil || node.StartByte() == node.EndByte() {
			continue
		}
		ranges = append(ranges, gotreesitter.HighlightRange{
			StartByte: node.StartByte(),
			EndByte:   node.EndByte(),
			Capture:   c.Name,
		})
	}
	if len(ranges) == 0 {
		return nil
	}

	resolved := resolveHighlightOverlaps(ranges)
	if len(resolved) == 0 {
		return nil
	}
	return renderer.normalizeResolvedRanges(len(tree.Source()), resolved)
}

func (renderer *treeSitterRenderer) normalizeResolvedRanges(codeLen int, ranges []gotreesitter.HighlightRange) []normalizedHighlightRange {
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
		n := len(out)
		if n > 0 && out[n-1].class == className && out[n-1].end == start {
			out[n-1].end = end
		} else {
			out = append(out, normalizedHighlightRange{start: start, end: end, class: className})
		}
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

func resolveHighlightOverlaps(ranges []gotreesitter.HighlightRange) []gotreesitter.HighlightRange {
	if len(ranges) == 0 {
		return nil
	}

	sorted := make([]gotreesitter.HighlightRange, 0, len(ranges))
	for i := range ranges {
		r := ranges[i]
		if r.EndByte > r.StartByte {
			sorted = append(sorted, r)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartByte != sorted[j].StartByte {
			return sorted[i].StartByte < sorted[j].StartByte
		}
		wi := sorted[i].EndByte - sorted[i].StartByte
		wj := sorted[j].EndByte - sorted[j].StartByte
		return wi > wj
	})

	stack := make([]gotreesitter.HighlightRange, 0, 32)
	result := make([]gotreesitter.HighlightRange, 0, len(sorted))
	emit := func(start, end uint32, capture string) {
		if capture == "" || end <= start {
			return
		}
		n := len(result)
		if n > 0 && result[n-1].Capture == capture && result[n-1].EndByte == start {
			result[n-1].EndByte = end
			return
		}
		result = append(result, gotreesitter.HighlightRange{
			StartByte: start,
			EndByte:   end,
			Capture:   capture,
		})
	}

	curPos := sorted[0].StartByte
	nextStartIdx := 0
	for nextStartIdx < len(sorted) || len(stack) > 0 {
		nextStartPos := ^uint32(0)
		if nextStartIdx < len(sorted) {
			nextStartPos = sorted[nextStartIdx].StartByte
		}
		nextEndPos := ^uint32(0)
		if len(stack) > 0 {
			nextEndPos = stack[len(stack)-1].EndByte
		}
		nextPos := min(nextStartPos, nextEndPos)

		if len(stack) > 0 && nextPos > curPos {
			emit(curPos, nextPos, stack[len(stack)-1].Capture)
			curPos = nextPos
		} else if curPos < nextPos {
			curPos = nextPos
		}

		// End events at this boundary are processed before start events.
		for len(stack) > 0 && stack[len(stack)-1].EndByte <= curPos {
			stack = stack[:len(stack)-1]
		}
		for nextStartIdx < len(sorted) && sorted[nextStartIdx].StartByte == curPos {
			stack = append(stack, sorted[nextStartIdx])
			nextStartIdx++
		}

		if len(stack) == 0 && nextStartIdx < len(sorted) && curPos < sorted[nextStartIdx].StartByte {
			curPos = sorted[nextStartIdx].StartByte
		}
		if len(stack) == 0 && nextStartIdx >= len(sorted) {
			break
		}
		if len(stack) > 0 && curPos < stack[len(stack)-1].StartByte {
			curPos = stack[len(stack)-1].StartByte
		}
		if len(stack) > 0 && curPos > stack[len(stack)-1].EndByte {
			for len(stack) > 0 && curPos >= stack[len(stack)-1].EndByte {
				stack = stack[:len(stack)-1]
			}
		}
	}

	return result
}

// normalizeHighlightRanges is a standalone version of normalizeResolvedRanges
// used by benchmarks that operate outside the renderer.
func normalizeHighlightRanges(codeLen int, ranges []gotreesitter.HighlightRange) []normalizedHighlightRange {
	out := make([]normalizedHighlightRange, 0, len(ranges))
	classByCapture := make(map[string]string, 32)
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

		className, ok := classByCapture[hr.Capture]
		if !ok {
			className = treeSitterCaptureToChromaClass(hr.Capture)
			classByCapture[hr.Capture] = className
		}
		n := len(out)
		if n > 0 && out[n-1].class == className && out[n-1].end == start {
			out[n-1].end = end
		} else {
			out = append(out, normalizedHighlightRange{start: start, end: end, class: className})
		}
		last = end
	}
	return out
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
