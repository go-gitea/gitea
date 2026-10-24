// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"fmt"
	"html"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"gitea.dev/modules/translation"
	"gitea.dev/modules/util"
)

type htmlEscaper struct {
	escaped         *EscapeStatus
	locale          translation.Locale
	ambiguousTables []*AmbiguousTable
	allowed         map[rune]bool

	inTagMath bool // MathML operators like U+2212 are intended and wrapping them breaks the math layout

	detectResults []detectResult
	written       int // content before this offset has been written to out
	out           strings.Builder
}

const detectWindowRunes = 4096 // bounds the detection results of long text parts

// escape returns content with the runes of its text parts that need escaping replaced, or content itself if there are none
func (e *htmlEscaper) escape(content string) string {
	for pos := 0; pos < len(content); {
		if content[pos] == '<' {
			tagEnd := len(content)
			if idx := strings.IndexByte(content[pos:], '>'); idx != -1 {
				tagEnd = pos + idx + 1
			}
			e.trackHtmlTag(content[pos:tagEnd])
			pos = tagEnd
			continue
		}
		textEnd := len(content)
		if idx := strings.IndexByte(content[pos:], '<'); idx != -1 {
			textEnd = pos + idx
		}
		text, _ := strings.CutPrefix(content[pos:textEnd], "\xef\xbb\xbf") // a leading BOM is kept as is
		if !e.inTagMath && !isStringAsciiSafe(text) {
			e.escapeText(content, textEnd-len(text), textEnd)
		}
		pos = textEnd
	}
	if e.written == 0 {
		return content
	}
	e.out.WriteString(content[e.written:])
	return e.out.String()
}

// escapeText analyzes content[start:end] in windows overlapping by the context analyzeDetectResults looks at
func (e *htmlEscaper) escapeText(content string, start, end int) {
	for skip := 0; ; skip = longSentenceDetectionLimit {
		results := e.detectRunes(content[start:end])
		e.analyzeDetectResults(results)
		lastResult := results[len(results)-1]
		isLastWindow := start+lastResult.position+lastResult.runeSize == end
		emitEnd := len(results)
		if !isLastWindow {
			emitEnd -= longSentenceDetectionLimit
		}
		for _, res := range results[skip:emitEnd] {
			if res.needEscape {
				e.out.WriteString(content[e.written : start+res.position])
				e.writeEscapedRune(res)
				e.written = start + res.position + res.runeSize
			}
		}
		if isLastWindow {
			return
		}
		start += results[emitEnd-longSentenceDetectionLimit].position
	}
}

func (e *htmlEscaper) trackHtmlTag(tag string) {
	isTag := func(prefix string) bool {
		if len(tag) < len(prefix)+1 {
			return false
		}
		if !util.AsciiEqualFold(tag[:len(prefix)], prefix) {
			return false
		}
		return strings.IndexByte(" \t\n\r\f>", tag[len(prefix)]) != -1
	}
	if isTag("<math") {
		e.inTagMath = true
	} else if isTag("</math") {
		e.inTagMath = false
	}
}

const longSentenceDetectionLimit = 20

func (e *htmlEscaper) possibleLongSentence(results []detectResult, pos int) bool {
	countBasic := 0
	countNonASCII := 0
	for i := max(pos-longSentenceDetectionLimit, 0); i < min(pos+longSentenceDetectionLimit, len(results)); i++ {
		if results[i].runeType == runeTypeBasic && results[i].runeChar != ' ' {
			countBasic++
		}
		if results[i].runeType == runeTypeNonASCII || results[i].runeType == runeTypeAmbiguous {
			countNonASCII++
		}
	}
	countChar := countBasic + countNonASCII
	// many non-ASCII runes around, it seems to be a sentence,
	// don't handle the invisible/ambiguous chars in it, otherwise it will be too noisy
	return countChar != 0 && countNonASCII*100/countChar >= 50
}

func (e *htmlEscaper) analyzeDetectResults(results []detectResult) {
	for i := range results {
		res := &results[i]
		if res.runeType == runeTypeInvisible || res.runeType == runeTypeAmbiguous {
			leftIsNonASCII := i > 0 && (results[i-1].runeType == runeTypeNonASCII || results[i-1].runeType == runeTypeAmbiguous)
			rightIsNonASCII := i < len(results)-1 && (results[i+1].runeType == runeTypeNonASCII || results[i+1].runeType == runeTypeAmbiguous)
			surroundingNonASCII := leftIsNonASCII || rightIsNonASCII
			res.needEscape = !surroundingNonASCII && (len(results) < longSentenceDetectionLimit || !e.possibleLongSentence(results, i))
		}
	}
}

func (e *htmlEscaper) writeEscapedRune(res detectResult) {
	switch res.runeType {
	case runeTypeBroken:
		e.writeBrokenRune()
	case runeTypeAmbiguous:
		e.writeAmbiguousRune(res.runeChar, res.confusable)
	case runeTypeInvisible:
		e.writeInvisibleRune(res.runeChar)
	case runeTypeControlChar:
		e.writeControlRune(res.runeChar)
	default:
		panic("unreachable")
	}
}

func (e *htmlEscaper) writeBrokenRune() {
	// Although we'd like to use the original bytes to display (show the real broken content to users),
	// however, when this escaper is applied to the content, the content has already been processed by other modules.
	// So the invalid bytes just can't be kept till this step, in most (all) cases, the only thing we see here is utf8.RuneError
	e.out.WriteString(`<span class="broken-code-point">�</span>`)
}

func (e *htmlEscaper) writeEscapedCharHTML(tag1, attr, tag2, content, tag3 string) {
	e.out.WriteString(tag1)
	e.out.WriteString(html.EscapeString(attr))
	e.out.WriteString(tag2)
	e.out.WriteString(html.EscapeString(content))
	e.out.WriteString(tag3)
}

func runeToHex(r rune) string {
	return fmt.Sprintf("[U+%04X]", r)
}

func (e *htmlEscaper) writeAmbiguousRune(r, c rune) {
	e.escaped.Escaped = true
	e.escaped.HasAmbiguous = true
	e.writeEscapedCharHTML(
		`<span class="ambiguous-code-point" data-tooltip-content="`,
		e.locale.TrString("repo.ambiguous_character", string(r)+" "+runeToHex(r), string(c)+" "+runeToHex(c)),
		`"><span class="char">`,
		string(r),
		`</span></span>`,
	)
}

func (e *htmlEscaper) writeInvisibleRune(r rune) {
	e.escaped.Escaped = true
	e.escaped.HasInvisible = true
	e.writeEscapedCharHTML(
		`<span class="escaped-code-point" data-escaped="`,
		runeToHex(r),
		`"><span class="char">`,
		string(r),
		`</span></span>`,
	)
}

func (e *htmlEscaper) writeControlRune(r rune) {
	var display string
	if r >= 0 && r <= 0x1f {
		display = string(0x2400 + r)
	} else if r == 0x7f {
		display = string(rune(0x2421))
	} else {
		display = runeToHex(r)
	}
	e.writeEscapedCharHTML(
		`<span class="broken-code-point" data-escaped="`,
		display,
		`"><span class="char">`,
		string(r),
		`</span></span>`,
	)
}

type detectResult struct {
	runeChar   rune
	runeType   int
	runeSize   int
	position   int
	confusable rune
	needEscape bool
}

const (
	runeTypeBasic int = iota
	runeTypeBroken
	runeTypeNonASCII
	runeTypeAmbiguous
	runeTypeInvisible
	runeTypeControlChar
)

func (e *htmlEscaper) detectRunes(data string) []detectResult {
	results := slices.Grow(e.detectResults[:0], min(len(data), detectWindowRunes))
	invisibleRangeTable := globalVars().invisibleRangeTable
	for pos := 0; pos < len(data) && len(results) < detectWindowRunes; {
		r, runeSize := utf8.DecodeRuneInString(data[pos:])
		res := detectResult{runeChar: r, runeSize: runeSize, position: pos}
		pos += runeSize

		switch {
		case r < utf8.RuneSelf && asciiSafeBytes[r]: // basic rune, no table contains printable ASCII
		case r == utf8.RuneError:
			res.runeType = runeTypeBroken
			res.needEscape = true
		case e.allowed[r]:
			res.runeType = runeTypeBasic
			if r >= 0x80 {
				res.runeType = runeTypeNonASCII
			}
		case r < 0x20 || r == 0x7f:
			res.runeType = runeTypeControlChar
			res.needEscape = true
		case unicode.Is(invisibleRangeTable, r):
			res.runeType = runeTypeInvisible
			// not sure about res.needEscape, will be detected separately
		case isAmbiguous(r, &res.confusable, e.ambiguousTables...):
			res.runeType = runeTypeAmbiguous
			// not sure about res.needEscape, will be detected separately
		default:
			res.runeType = runeTypeNonASCII
		}
		results = append(results, res)
	}
	e.detectResults = results
	return results
}
