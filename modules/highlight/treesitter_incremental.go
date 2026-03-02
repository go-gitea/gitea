// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package highlight

import (
	"bytes"
	"html/template"

	"github.com/odvcencio/gotreesitter"
)

type treeSitterRenderCache struct {
	source []byte

	code template.HTML
	// trim tracks whether code cache was produced with trailing newline trim.
	trim bool

	lines  []template.HTML
	ranges []normalizedHighlightRange
	tree   *gotreesitter.Tree

	altSource []byte
	altCode   template.HTML
	altTrim   bool
	altLines  []template.HTML
}

func (c *treeSitterRenderCache) matches(code []byte) bool {
	if len(c.source) != len(code) {
		return false
	}
	if len(code) == 0 {
		return true
	}
	// Fast reject: most misses differ near boundaries.
	if c.source[0] != code[0] || c.source[len(code)-1] != code[len(code)-1] {
		return false
	}
	// Additional boundary guards to avoid full slice compare on common misses.
	if len(code) > 16 {
		if !bytes.Equal(c.source[:8], code[:8]) {
			return false
		}
		if !bytes.Equal(c.source[len(code)-8:], code[len(code)-8:]) {
			return false
		}
	}
	return bytes.Equal(c.source, code)
}

func (c *treeSitterRenderCache) matchesAlt(code []byte) bool {
	if len(c.altSource) != len(code) {
		return false
	}
	if len(code) == 0 {
		return true
	}
	if c.altSource[0] != code[0] || c.altSource[len(code)-1] != code[len(code)-1] {
		return false
	}
	if len(code) > 16 {
		if !bytes.Equal(c.altSource[:8], code[:8]) {
			return false
		}
		if !bytes.Equal(c.altSource[len(code)-8:], code[len(code)-8:]) {
			return false
		}
	}
	return bytes.Equal(c.altSource, code)
}

func (c *treeSitterRenderCache) setSource(code []byte) {
	if cap(c.source) < len(code) {
		c.source = make([]byte, len(code))
	} else {
		c.source = c.source[:len(code)]
	}
	copy(c.source, code)
}

func (c *treeSitterRenderCache) setAltSource(code []byte) {
	if cap(c.altSource) < len(code) {
		c.altSource = make([]byte, len(code))
	} else {
		c.altSource = c.altSource[:len(code)]
	}
	copy(c.altSource, code)
}

func (c *treeSitterRenderCache) setRanges(ranges []normalizedHighlightRange) {
	c.ranges = ranges
}

func (c *treeSitterRenderCache) setTree(tree *gotreesitter.Tree) {
	if c.tree != nil && c.tree != tree {
		c.tree.Release()
	}
	c.tree = tree
}

func (c *treeSitterRenderCache) clearDerived() {
	c.code = ""
	c.trim = false
	c.lines = nil
}

func (c *treeSitterRenderCache) archiveCurrentDerivedToAlt() {
	if len(c.source) == 0 {
		c.altSource = c.altSource[:0]
		c.altCode = ""
		c.altTrim = false
		c.altLines = nil
		return
	}
	c.setAltSource(c.source)
	c.altCode = c.code
	c.altTrim = c.trim
	if c.lines != nil {
		c.altLines = append([]template.HTML(nil), c.lines...)
	} else {
		c.altLines = nil
	}
}

func computeSingleInputEdit(oldSource, newSource []byte) (gotreesitter.InputEdit, bool) {
	if bytes.Equal(oldSource, newSource) {
		return gotreesitter.InputEdit{}, false
	}

	minLen := min(len(oldSource), len(newSource))
	prefix := 0
	for prefix < minLen && oldSource[prefix] == newSource[prefix] {
		prefix++
	}

	oldRemain := len(oldSource) - prefix
	newRemain := len(newSource) - prefix
	suffix := 0
	for suffix < oldRemain && suffix < newRemain &&
		oldSource[len(oldSource)-1-suffix] == newSource[len(newSource)-1-suffix] {
		suffix++
	}

	oldEnd := len(oldSource) - suffix
	newEnd := len(newSource) - suffix

	startPoint := pointAtOffset(oldSource, prefix)
	oldEndPoint := pointAtOffset(oldSource, oldEnd)
	newEndPoint := pointAtOffset(newSource, newEnd)

	return gotreesitter.InputEdit{
		StartByte:   uint32(prefix),
		OldEndByte:  uint32(oldEnd),
		NewEndByte:  uint32(newEnd),
		StartPoint:  startPoint,
		OldEndPoint: oldEndPoint,
		NewEndPoint: newEndPoint,
	}, true
}

func pointAtOffset(source []byte, offset int) gotreesitter.Point {
	if offset <= 0 {
		return gotreesitter.Point{}
	}
	if offset > len(source) {
		offset = len(source)
	}
	var row, col uint32
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			row++
			col = 0
			continue
		}
		col++
	}
	return gotreesitter.Point{Row: row, Column: col}
}
