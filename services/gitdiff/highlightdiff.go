// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// token is a html tag or entity, eg: "<span ...>", "</span>", "&lt;"
func extractHTMLToken(s string) (before, token, after string, valid bool) {
	for pos1 := 0; pos1 < len(s); pos1++ {
		if s[pos1] == '<' {
			pos2 := strings.IndexByte(s[pos1:], '>')
			if pos2 == -1 {
				return "", "", s, false
			}
			return s[:pos1], s[pos1 : pos1+pos2+1], s[pos1+pos2+1:], true
		} else if s[pos1] == '&' {
			pos2 := strings.IndexByte(s[pos1:], ';')
			if pos2 == -1 {
				return "", "", s, false
			}
			return s[:pos1], s[pos1 : pos1+pos2+1], s[pos1+pos2+1:], true
		}
	}
	return "", "", s, true
}

// highlightCodeDiff is used to do diff with highlighted HTML code.
// It totally depends on Chroma's valid HTML output and its structure, do not use these functions for other purposes.
// The HTML tags and entities will be replaced by Unicode placeholders: "<span>{TEXT}</span>" => "\uE000{TEXT}\uE001"
// These Unicode placeholders are friendly to the diff.
// Then after diff, the placeholders in diff result will be recovered to the HTML tags and entities.
// It's guaranteed that the tags in final diff result are paired correctly.
type highlightCodeDiff struct {
	placeholderBegin    rune
	placeholderMaxCount int
	placeholderIndex    int
	placeholderTokenMap map[rune]string
	tokenPlaceholderMap map[string]rune

	placeholderOverflowCount int

	lineWrapperTags []string
}

func newHighlightCodeDiff() *highlightCodeDiff {
	return &highlightCodeDiff{
		placeholderBegin:    rune(0x100000), // Plane 16: Supplementary Private Use Area B (U+100000..U+10FFFD)
		placeholderMaxCount: 64000,
		placeholderTokenMap: map[rune]string{},
		tokenPlaceholderMap: map[string]rune{},
	}
}

// nextPlaceholder returns 0 if no more placeholder can be used
// the diff is done line by line, usually there are only a few (no more than 10) placeholders in one line
// so the placeholderMaxCount is impossible to be exhausted in real cases.
func (hcd *highlightCodeDiff) nextPlaceholder() rune {
	for hcd.placeholderIndex < hcd.placeholderMaxCount {
		r := hcd.placeholderBegin + rune(hcd.placeholderIndex)
		hcd.placeholderIndex++
		// only use non-existing (not used by code) rune as placeholders
		if _, ok := hcd.placeholderTokenMap[r]; !ok {
			return r
		}
	}
	return 0 // no more available placeholder
}

func (hcd *highlightCodeDiff) isInPlaceholderRange(r rune) bool {
	return hcd.placeholderBegin <= r && r < hcd.placeholderBegin+rune(hcd.placeholderMaxCount)
}

func (hcd *highlightCodeDiff) collectUsedRunes(code template.HTML) {
	for _, r := range code {
		if hcd.isInPlaceholderRange(r) {
			// put the existing rune (used by code) in map, then this rune won't be used a placeholder anymore.
			hcd.placeholderTokenMap[r] = ""
		}
	}
}

func (hcd *highlightCodeDiff) diffLineWithHighlight(lineType DiffLineType, codeA, codeB template.HTML) template.HTML {
	return hcd.diffLineWithHighlightWrapper(nil, lineType, codeA, codeB)
}

func (hcd *highlightCodeDiff) diffLineWithHighlightWrapper(lineWrapperTags []string, lineType DiffLineType, codeA, codeB template.HTML) template.HTML {
	hcd.collectUsedRunes(codeA)
	hcd.collectUsedRunes(codeB)

	convertedCodeA := hcd.convertToPlaceholders(codeA)
	convertedCodeB := hcd.convertToPlaceholders(codeB)

	dmp := defaultDiffMatchPatch()
	diffs := dmp.DiffMain(convertedCodeA, convertedCodeB, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	buf := bytes.NewBuffer(nil)

	// restore the line wrapper tags <span class="line"> and <span class="cl">, if necessary
	for _, tag := range lineWrapperTags {
		buf.WriteString(tag)
	}

	addedCodePrefix := hcd.registerTokenAsPlaceholder(`<span class="added-code">`)
	removedCodePrefix := hcd.registerTokenAsPlaceholder(`<span class="removed-code">`)
	codeTagSuffix := hcd.registerTokenAsPlaceholder(`</span>`)

	if codeTagSuffix != 0 {
		for _, diff := range diffs {
			switch {
			case diff.Type == diffmatchpatch.DiffEqual:
				buf.WriteString(diff.Text)
			case diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
				buf.WriteRune(addedCodePrefix)
				buf.WriteString(diff.Text)
				buf.WriteRune(codeTagSuffix)
			case diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
				buf.WriteRune(removedCodePrefix)
				buf.WriteString(diff.Text)
				buf.WriteRune(codeTagSuffix)
			}
		}
	} else {
		// placeholder map space is exhausted
		for _, diff := range diffs {
			take := diff.Type == diffmatchpatch.DiffEqual || (diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd) || (diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel)
			if take {
				buf.WriteString(diff.Text)
			}
		}
	}
	for range lineWrapperTags {
		buf.WriteString("</span>")
	}
	return hcd.recoverOneDiff(buf.String())
}

func (hcd *highlightCodeDiff) registerTokenAsPlaceholder(token string) rune {
	placeholder, ok := hcd.tokenPlaceholderMap[token]
	if !ok {
		placeholder = hcd.nextPlaceholder()
		if placeholder != 0 {
			hcd.tokenPlaceholderMap[token] = placeholder
			hcd.placeholderTokenMap[placeholder] = token
		}
	}
	return placeholder
}

// convertToPlaceholders totally depends on Chroma's valid HTML output and its structure, do not use these functions for other purposes.
func (hcd *highlightCodeDiff) convertToPlaceholders(htmlContent template.HTML) string {
	var tagStack []string
	res := strings.Builder{}

	firstRunForLineTags := hcd.lineWrapperTags == nil

	var beforeToken, token string
	var valid bool

	htmlCode := string(htmlContent)
	// the standard chroma highlight HTML is "<span class="line [hl]"><span class="cl"> ... </span></span>"
	for {
		beforeToken, token, htmlCode, valid = extractHTMLToken(htmlCode)
		if !valid || token == "" {
			break
		}
		// write the content before the token into result string, and consume the token in the string
		res.WriteString(beforeToken)

		// the line wrapper tags should be removed before diff
		if strings.HasPrefix(token, `<span class="line`) || strings.HasPrefix(token, `<span class="cl"`) {
			if firstRunForLineTags {
				// if this is the first run for converting, save the line wrapper tags for later use, they should be added back
				hcd.lineWrapperTags = append(hcd.lineWrapperTags, token)
			}
			htmlCode = strings.TrimSuffix(htmlCode, "</span>")
			continue
		}

		var tokenInMap string
		if strings.HasSuffix(token, "</") { // for closing tag
			if len(tagStack) == 0 {
				break // invalid diff result, no opening tag but see closing tag
			}
			// make sure the closing tag in map is related to the open tag, to make the diff algorithm can match the opening/closing tags
			// the closing tag will be recorded in the map by key "</span><!-- <span the-opening> -->" for "<span the-opening>"
			tokenInMap = token + "<!-- " + tagStack[len(tagStack)-1] + "-->"
			tagStack = tagStack[:len(tagStack)-1]
		} else if token[0] == '<' { // for opening tag
			tokenInMap = token
			tagStack = append(tagStack, token)
		} else if token[0] == '&' { // for html entity
			tokenInMap = token
		} // else: impossible

		// remember the placeholder and token in the map
		placeholder := hcd.registerTokenAsPlaceholder(tokenInMap)

		if placeholder != 0 {
			res.WriteRune(placeholder) // use the placeholder to replace the token
		} else {
			// unfortunately, all private use runes has been exhausted, no more placeholder could be used, no more converting
			// usually, the exhausting won't occur in real cases, the magnitude of used placeholders is not larger than that of the CSS classes outputted by chroma.
			hcd.placeholderOverflowCount++
			if strings.HasPrefix(token, "&") {
				// when the token is a html entity, something must be outputted even if there is no placeholder.
				res.WriteRune(0xFFFD)      // replacement character TODO: how to handle this case more gracefully?
				res.WriteString(token[1:]) // still output the entity code part, otherwise there will be no diff result.
			}
		}
	}

	// write the remaining string
	res.WriteString(htmlCode)
	return res.String()
}

func (hcd *highlightCodeDiff) recoverOneDiff(str string) template.HTML {
	sb := strings.Builder{}
	var tagStack []string

	for _, r := range str {
		token, ok := hcd.placeholderTokenMap[r]
		if !ok || token == "" {
			sb.WriteRune(r) // if the rune is not a placeholder, write it as it is
			continue
		}
		var tokenToRecover string
		if strings.HasPrefix(token, "</") { // for closing tag
			// only get the tag itself, ignore the trailing comment (for how the comment is generated, see the code in `convert` function)
			tokenToRecover = token[:strings.IndexByte(token, '>')+1]
			if len(tagStack) == 0 {
				continue // if no opening tag in stack yet, skip the closing tag
			}
			tagStack = tagStack[:len(tagStack)-1]
		} else if token[0] == '<' { // for opening tag
			tokenToRecover = token
			tagStack = append(tagStack, token)
		} else if token[0] == '&' { // for html entity
			tokenToRecover = token
		} // else: impossible
		sb.WriteString(tokenToRecover)
	}

	if len(tagStack) > 0 {
		// close all opening tags
		for i := len(tagStack) - 1; i >= 0; i-- {
			tagToClose := tagStack[i]
			// get the closing tag "</span>" from "<span class=...>" or "<span>"
			pos := strings.IndexAny(tagToClose, " >")
			if pos != -1 {
				sb.WriteString("</" + tagToClose[1:pos] + ">")
			} // else: impossible. every tag was pushed into the stack by the code above and is valid HTML opening tag
		}
	}
	return template.HTML(sb.String())
}
