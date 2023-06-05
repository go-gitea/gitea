// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/translation"

	"golang.org/x/net/html"
)

// VScode defaultWordRegexp
var defaultWordRegexp = regexp.MustCompile(`(-?\d*\.\d\w*)|([^\` + "`" + `\~\!\@\#\$\%\^\&\*\(\)\-\=\+\[\{\]\}\\\|\;\:\'\"\,\.\<\>\/\?\s\x00-\x1f]+)`)

func NewEscapeStreamer(locale translation.Locale, next HTMLStreamer, allowed ...rune) HTMLStreamer {
	allowedM := make(map[rune]bool, len(allowed))
	for _, v := range allowed {
		allowedM[v] = true
	}
	return &escapeStreamer{
		escaped:                 &EscapeStatus{},
		PassthroughHTMLStreamer: *NewPassthroughStreamer(next),
		locale:                  locale,
		ambiguousTables:         AmbiguousTablesForLocale(locale),
		allowed:                 allowedM,
	}
}

type escapeStreamer struct {
	PassthroughHTMLStreamer
	escaped         *EscapeStatus
	locale          translation.Locale
	ambiguousTables []*AmbiguousTable
	allowed         map[rune]bool
}

func (e *escapeStreamer) EscapeStatus() *EscapeStatus {
	return e.escaped
}

// Text tells the next streamer there is a text
func (e *escapeStreamer) Text(data string) error {
	sb := &strings.Builder{}
	var until int
	var next int
	pos := 0
	if len(data) > len(UTF8BOM) && data[:len(UTF8BOM)] == string(UTF8BOM) {
		_, _ = sb.WriteString(data[:len(UTF8BOM)])
		pos = len(UTF8BOM)
	}
	dataBytes := []byte(data)
	for pos < len(data) {
		nextIdxs := defaultWordRegexp.FindStringIndex(data[pos:])
		if nextIdxs == nil {
			until = len(data)
			next = until
		} else {
			until, next = nextIdxs[0]+pos, nextIdxs[1]+pos
		}

		// from pos until until we know that the runes are not \r\t\n or even ' '
		runes := make([]rune, 0, next-until)
		positions := make([]int, 0, next-until+1)

		for pos < until {
			r, sz := utf8.DecodeRune(dataBytes[pos:])
			positions = positions[:0]
			positions = append(positions, pos, pos+sz)
			types, confusables, _ := e.runeTypes(r)
			if err := e.handleRunes(dataBytes, []rune{r}, positions, types, confusables, sb); err != nil {
				return err
			}
			pos += sz
		}

		for i := pos; i < next; {
			r, sz := utf8.DecodeRune(dataBytes[i:])
			runes = append(runes, r)
			positions = append(positions, i)
			i += sz
		}
		positions = append(positions, next)
		types, confusables, runeCounts := e.runeTypes(runes...)
		if runeCounts.needsEscape() {
			if err := e.handleRunes(dataBytes, runes, positions, types, confusables, sb); err != nil {
				return err
			}
		} else {
			_, _ = sb.Write(dataBytes[pos:next])
		}
		pos = next
	}
	if sb.Len() > 0 {
		if err := e.PassthroughHTMLStreamer.Text(sb.String()); err != nil {
			return err
		}
	}
	return nil
}

func (e *escapeStreamer) handleRunes(data []byte, runes []rune, positions []int, types []runeType, confusables []rune, sb *strings.Builder) error {
	for i, r := range runes {
		switch types[i] {
		case brokenRuneType:
			if sb.Len() > 0 {
				if err := e.PassthroughHTMLStreamer.Text(sb.String()); err != nil {
					return err
				}
				sb.Reset()
			}
			end := positions[i+1]
			start := positions[i]
			if err := e.brokenRune(data[start:end]); err != nil {
				return err
			}
		case ambiguousRuneType:
			if sb.Len() > 0 {
				if err := e.PassthroughHTMLStreamer.Text(sb.String()); err != nil {
					return err
				}
				sb.Reset()
			}
			if err := e.ambiguousRune(r, confusables[0]); err != nil {
				return err
			}
			confusables = confusables[1:]
		case invisibleRuneType:
			if sb.Len() > 0 {
				if err := e.PassthroughHTMLStreamer.Text(sb.String()); err != nil {
					return err
				}
				sb.Reset()
			}
			if err := e.invisibleRune(r); err != nil {
				return err
			}
		default:
			_, _ = sb.WriteRune(r)
		}
	}
	return nil
}

func (e *escapeStreamer) brokenRune(bs []byte) error {
	e.escaped.Escaped = true
	e.escaped.HasBadRunes = true

	if err := e.PassthroughHTMLStreamer.StartTag("span", html.Attribute{
		Key: "class",
		Val: "broken-code-point",
	}); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.Text(fmt.Sprintf("<%X>", bs)); err != nil {
		return err
	}

	return e.PassthroughHTMLStreamer.EndTag("span")
}

func (e *escapeStreamer) ambiguousRune(r, c rune) error {
	e.escaped.Escaped = true
	e.escaped.HasAmbiguous = true

	if err := e.PassthroughHTMLStreamer.StartTag("span", html.Attribute{
		Key: "class",
		Val: "ambiguous-code-point",
	}, html.Attribute{
		Key: "data-tooltip-content",
		Val: e.locale.Tr("repo.ambiguous_character", r, c),
	}); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.StartTag("span", html.Attribute{
		Key: "class",
		Val: "char",
	}); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.Text(string(r)); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.EndTag("span"); err != nil {
		return err
	}

	return e.PassthroughHTMLStreamer.EndTag("span")
}

func (e *escapeStreamer) invisibleRune(r rune) error {
	e.escaped.Escaped = true
	e.escaped.HasInvisible = true

	if err := e.PassthroughHTMLStreamer.StartTag("span", html.Attribute{
		Key: "class",
		Val: "escaped-code-point",
	}, html.Attribute{
		Key: "data-escaped",
		Val: fmt.Sprintf("[U+%04X]", r),
	}); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.StartTag("span", html.Attribute{
		Key: "class",
		Val: "char",
	}); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.Text(string(r)); err != nil {
		return err
	}
	if err := e.PassthroughHTMLStreamer.EndTag("span"); err != nil {
		return err
	}

	return e.PassthroughHTMLStreamer.EndTag("span")
}

type runeCountType struct {
	numBasicRunes                int
	numNonConfusingNonBasicRunes int
	numAmbiguousRunes            int
	numInvisibleRunes            int
	numBrokenRunes               int
}

func (counts runeCountType) needsEscape() bool {
	if counts.numBrokenRunes > 0 {
		return true
	}
	if counts.numBasicRunes == 0 &&
		counts.numNonConfusingNonBasicRunes > 0 {
		return false
	}
	return counts.numAmbiguousRunes > 0 || counts.numInvisibleRunes > 0
}

type runeType int

const (
	basicASCIIRuneType runeType = iota // <- This is technically deadcode but its self-documenting so it should stay
	brokenRuneType
	nonBasicASCIIRuneType
	ambiguousRuneType
	invisibleRuneType
)

func (e *escapeStreamer) runeTypes(runes ...rune) (types []runeType, confusables []rune, runeCounts runeCountType) {
	types = make([]runeType, len(runes))
	for i, r := range runes {
		var confusable rune
		switch {
		case r == utf8.RuneError:
			types[i] = brokenRuneType
			runeCounts.numBrokenRunes++
		case r == ' ' || r == '\t' || r == '\n':
			runeCounts.numBasicRunes++
		case e.allowed[r]:
			if r > 0x7e || r < 0x20 {
				types[i] = nonBasicASCIIRuneType
				runeCounts.numNonConfusingNonBasicRunes++
			} else {
				runeCounts.numBasicRunes++
			}
		case unicode.Is(InvisibleRanges, r):
			types[i] = invisibleRuneType
			runeCounts.numInvisibleRunes++
		case unicode.IsControl(r):
			types[i] = invisibleRuneType
			runeCounts.numInvisibleRunes++
		case isAmbiguous(r, &confusable, e.ambiguousTables...):
			confusables = append(confusables, confusable)
			types[i] = ambiguousRuneType
			runeCounts.numAmbiguousRunes++
		case r > 0x7e || r < 0x20:
			types[i] = nonBasicASCIIRuneType
			runeCounts.numNonConfusingNonBasicRunes++
		default:
			runeCounts.numBasicRunes++
		}
	}
	return types, confusables, runeCounts
}
