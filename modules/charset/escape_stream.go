// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"unicode"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
)

type htmlChunkReader struct {
	in       io.Reader
	readErr  error
	readBuf  []byte
	curInTag bool
}

type escapeStreamer struct {
	htmlChunkReader

	escaped         *EscapeStatus
	locale          translation.Locale
	ambiguousTables []*AmbiguousTable
	allowed         map[rune]bool

	out io.Writer
}

func escapeStream(locale translation.Locale, in io.Reader, out io.Writer, opts ...EscapeOptions) (*EscapeStatus, error) {
	es := &escapeStreamer{
		escaped:         &EscapeStatus{},
		locale:          locale,
		ambiguousTables: AmbiguousTablesForLocale(locale),
		htmlChunkReader: htmlChunkReader{
			in:      in,
			readBuf: make([]byte, 0, 32*1024),
		},
		out: out,
	}

	if len(opts) > 0 {
		es.allowed = opts[0].Allowed
	}

	readCount := 0
	lastIsTag := false
	for {
		parts, partInTag, err := es.readRunes()
		readCount++
		if err == io.EOF {
			return es.escaped, nil
		} else if err != nil {
			return nil, err
		}
		for i, part := range parts {
			if partInTag[i] {
				lastIsTag = true
				if _, err := out.Write(part); err != nil {
					return nil, err
				}
			} else {
				// if last part is tag, then this part is content begin
				// if the content is the first part of the first read, then it's also content begin
				isContentBegin := lastIsTag || (readCount == 1 && i == 0)
				lastIsTag = false
				if isContentBegin {
					if part, err = es.trimAndWriteBom(part); err != nil {
						return nil, err
					}
				}
				if err = es.detectAndWriteRunes(part); err != nil {
					return nil, err
				}
			}
		}
	}
}

func (e *escapeStreamer) trimAndWriteBom(part []byte) ([]byte, error) {
	remaining, ok := bytes.CutPrefix(part, globalVars().utf8Bom)
	if ok {
		part = remaining
		if _, err := e.out.Write(globalVars().utf8Bom); err != nil {
			return part, err
		}
	}
	return part, nil
}

const longSentenceDetectionLimit = 20

func (e *escapeStreamer) possibleLongSentence(results []detectResult, pos int) bool {
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

func (e *escapeStreamer) analyzeDetectResults(results []detectResult) {
	for i := range results {
		res := &results[i]
		if res.runeType == runeTypeInvisible || res.runeType == runeTypeAmbiguous {
			leftIsNonASCII := i > 0 && (results[i-1].runeType == runeTypeNonASCII || results[i-1].runeType == runeTypeAmbiguous)
			rightIsNonASCII := i < len(results)-1 && (results[i+1].runeType == runeTypeNonASCII || results[i+1].runeType == runeTypeAmbiguous)
			surroundingNonASCII := leftIsNonASCII || rightIsNonASCII
			if !surroundingNonASCII {
				if len(results) < longSentenceDetectionLimit {
					res.needEscape = setting.UI.AmbiguousUnicodeDetection
				} else if !e.possibleLongSentence(results, i) {
					res.needEscape = setting.UI.AmbiguousUnicodeDetection
				}
			}
		}
	}
}

func (e *escapeStreamer) detectAndWriteRunes(part []byte) error {
	results := e.detectRunes(part)
	e.analyzeDetectResults(results)
	return e.writeDetectResults(part, results)
}

func (e *htmlChunkReader) readRunes() (parts [][]byte, partInTag []bool, _ error) {
	// we have read everything, eof
	if e.readErr != nil && len(e.readBuf) == 0 {
		return nil, nil, e.readErr
	}

	// not eof, and the there is space in the buffer, try to read more data
	if e.readErr == nil && len(e.readBuf) <= cap(e.readBuf)*3/4 {
		n, err := e.in.Read(e.readBuf[len(e.readBuf):cap(e.readBuf)])
		e.readErr = err
		e.readBuf = e.readBuf[:len(e.readBuf)+n]
	}
	if len(e.readBuf) == 0 {
		return nil, nil, e.readErr
	}

	// try to exact tag parts and content parts
	pos := 0
	for pos < len(e.readBuf) {
		var curPartEnd int
		nextInTag := e.curInTag
		if e.curInTag {
			// if cur part is in tag, try to find the tag close char '>'
			idx := bytes.IndexByte(e.readBuf[pos:], '>')
			if idx == -1 {
				// if no tag close char, then the whole buffer is in tag
				curPartEnd = len(e.readBuf)
			} else {
				// tag part ends, switch to content part
				curPartEnd = pos + idx + 1
				nextInTag = !nextInTag
			}
		} else {
			// if cur part is in content, try to find the tag open char '<'
			idx := bytes.IndexByte(e.readBuf[pos:], '<')
			if idx == -1 {
				// if no tag open char, then the whole buffer is in content
				curPartEnd = len(e.readBuf)
			} else {
				// content part ends, switch to tag part
				curPartEnd = pos + idx
				nextInTag = !nextInTag
			}
		}

		curPartLen := curPartEnd - pos
		if curPartLen == 0 {
			// if cur part is empty, only need to switch the part type
			if e.curInTag == nextInTag {
				panic("impossible, curPartLen is 0 but the part in tag status is not switched")
			}
			e.curInTag = nextInTag
			continue
		}

		// now, curPartLen can't be 0
		curPart := make([]byte, curPartLen)
		copy(curPart, e.readBuf[pos:curPartEnd])
		// now we get the curPart bytes, but we can't directly use it, the last rune in it might have been cut
		// try to decode the last rune, if it's invalid, then we cut the last byte and try again until we get a valid rune or no byte left
		for i := curPartLen - 1; i >= 0; i-- {
			last, lastSize := utf8.DecodeRune(curPart[i:])
			if last == utf8.RuneError && lastSize == 1 {
				curPartLen--
			} else {
				curPartLen += lastSize - 1
				break
			}
		}
		if curPartLen == 0 {
			// actually it's impossible that the part doesn't contain any valid rune,
			// the only case is that the cap(readBuf) is too small, or the origin contain indeed doesn't contain any valid rune
			// * try to leave the last 4 bytes (possible longest utf-8 encoding) to next round
			// * at least consume 1 byte to avoid infinite loop
			curPartLen = max(len(curPart)-utf8.UTFMax, 1)
		}

		// if curPartLen is not the same as curPart, it means we have cut some bytes,
		// need to wait for more data if not eof
		trailingCorrupted := curPartLen != len(curPart)

		// finally, we get the real part we need
		curPart = curPart[:curPartLen]
		parts = append(parts, curPart)
		partInTag = append(partInTag, e.curInTag)

		pos += curPartLen
		e.curInTag = nextInTag

		if trailingCorrupted && e.readErr == nil {
			// if the last part is corrupted, and we haven't reach eof, then we need to wait for more data to get the complete part
			break
		}
	}

	copy(e.readBuf, e.readBuf[pos:])
	e.readBuf = e.readBuf[:len(e.readBuf)-pos]
	return parts, partInTag, nil
}

func (e *escapeStreamer) writeDetectResults(data []byte, results []detectResult) error {
	lastWriteRawIdx := -1
	for idx := range results {
		res := &results[idx]
		if !res.needEscape {
			if lastWriteRawIdx == -1 {
				lastWriteRawIdx = idx
			}
			continue
		}

		if lastWriteRawIdx != -1 {
			if _, err := e.out.Write(data[results[lastWriteRawIdx].position:res.position]); err != nil {
				return err
			}
			lastWriteRawIdx = -1
		}
		switch res.runeType {
		case runeTypeBroken:
			if err := e.writeBrokenRune(data[res.position : res.position+res.runeSize]); err != nil {
				return err
			}
		case runeTypeAmbiguous:
			if err := e.writeAmbiguousRune(res.runeChar, res.confusable); err != nil {
				return err
			}
		case runeTypeInvisible:
			if err := e.writeInvisibleRune(res.runeChar); err != nil {
				return err
			}
		case runeTypeControlChar:
			if err := e.writeControlRune(res.runeChar); err != nil {
				return err
			}
		default:
			panic("unreachable")
		}
	}
	if lastWriteRawIdx != -1 {
		lastResult := results[len(results)-1]
		if _, err := e.out.Write(data[results[lastWriteRawIdx].position : lastResult.position+lastResult.runeSize]); err != nil {
			return err
		}
	}
	return nil
}

func (e *escapeStreamer) writeBrokenRune(_ []byte) (err error) {
	// Although we'd like to use the original bytes to display (show the real broken content to users),
	// however, when this "escape stream" module is applied to the content, the content has already been processed by other modules.
	// So the invalid bytes just can't be kept till this step, in most (all) cases, the only thing we see here is utf8.RuneError
	_, err = io.WriteString(e.out, `<span class="broken-code-point">�</span>`)
	return err
}

func (e *escapeStreamer) writeEscapedCharHTML(tag1, attr, tag2, content, tag3 string) (err error) {
	_, err = io.WriteString(e.out, tag1)
	if err != nil {
		return err
	}
	_, err = io.WriteString(e.out, html.EscapeString(attr))
	if err != nil {
		return err
	}
	_, err = io.WriteString(e.out, tag2)
	if err != nil {
		return err
	}
	_, err = io.WriteString(e.out, html.EscapeString(content))
	if err != nil {
		return err
	}
	_, err = io.WriteString(e.out, tag3)
	return err
}

func runeToHex(r rune) string {
	return fmt.Sprintf("[U+%04X]", r)
}

func (e *escapeStreamer) writeAmbiguousRune(r, c rune) (err error) {
	e.escaped.Escaped = true
	e.escaped.HasAmbiguous = true
	return e.writeEscapedCharHTML(
		`<span class="ambiguous-code-point" data-tooltip-content="`,
		e.locale.TrString("repo.ambiguous_character", string(r)+" "+runeToHex(r), string(c)+" "+runeToHex(c)),
		`"><span class="char">`,
		string(r),
		`</span></span>`,
	)
}

func (e *escapeStreamer) writeInvisibleRune(r rune) error {
	e.escaped.Escaped = true
	e.escaped.HasInvisible = true
	return e.writeEscapedCharHTML(
		`<span class="escaped-code-point" data-escaped="`,
		runeToHex(r),
		`"><span class="char">`,
		string(r),
		`</span></span>`,
	)
}

func (e *escapeStreamer) writeControlRune(r rune) error {
	var display string
	if r >= 0 && r <= 0x1f {
		display = string(0x2400 + r)
	} else if r == 0x7f {
		display = string(rune(0x2421))
	} else {
		display = runeToHex(r)
	}
	return e.writeEscapedCharHTML(
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

func (e *escapeStreamer) detectRunes(data []byte) []detectResult {
	runeCount := utf8.RuneCount(data)
	results := make([]detectResult, runeCount)
	invisibleRangeTable := globalVars().invisibleRangeTable
	var i int
	var confusable rune
	for pos := 0; pos < len(data); i++ {
		r, runeSize := utf8.DecodeRune(data[pos:])
		results[i].runeChar = r
		results[i].runeSize = runeSize
		results[i].position = pos
		pos += runeSize

		switch {
		case r == utf8.RuneError:
			results[i].runeType = runeTypeBroken
			results[i].needEscape = true
		case r == ' ' || r == '\t' || r == '\n' || e.allowed[r]:
			results[i].runeType = runeTypeBasic
			if r >= 0x80 {
				results[i].runeType = runeTypeNonASCII
			}
		case r < 0x20 || r == 0x7f:
			results[i].runeType = runeTypeControlChar
			results[i].needEscape = true
		case unicode.Is(invisibleRangeTable, r):
			results[i].runeType = runeTypeInvisible
			// not sure about results[i].needEscape, will be detected separately
		case isAmbiguous(r, &confusable, e.ambiguousTables...):
			results[i].runeType = runeTypeAmbiguous
			results[i].confusable = confusable
			// not sure about results[i].needEscape, will be detected separately
		case r >= 0x80:
			results[i].runeType = runeTypeNonASCII
		default: // details to basic runes
		}
	}
	return results
}
