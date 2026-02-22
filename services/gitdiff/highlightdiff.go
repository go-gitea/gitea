// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitdiff

import (
	"bytes"
	"html/template"
	"strings"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/util"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// extractDiffTokenRemainingFullTag tries to extract full tag with content from the remaining string
// e.g. for input: "content</span>the-rest...", it returns "content</span>", "the-rest...", true
func extractDiffTokenRemainingFullTag(s string) (token, after string, valid bool) {
	pos := 0
	for ; pos < len(s); pos++ {
		c := s[pos]
		if c == '<' {
			break
		}
		// keep in mind: even if we'd like to relax this check,
		// we should never ignore "&" because it is for HTML entity and can't be safely used in the diff algorithm,
		// because diff between "&lt;" and "&gt;" will generate broken result.
		isSymbolChar := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '_' || c == '-' || c == '.'
		if !isSymbolChar {
			return "", s, false
		}
	}
	if pos+1 >= len(s) || s[pos+1] != '/' {
		return "", s, false
	}
	pos2 := strings.IndexByte(s[pos:], '>')
	if pos2 == -1 {
		return "", s, false
	}
	return s[:pos+pos2+1], s[pos+pos2+1:], true
}

// Returned token:
// * full tag with content: "<<span>content</span>>", it is used to optimize diff results to highlight the whole changed symbol
// * opening/closing tag: "<span ...>" or "</span>"
// * HTML entity: "&lt;"
func extractDiffToken(s string) (before, token, after string, valid bool) {
	for pos1 := 0; pos1 < len(s); pos1++ {
		switch s[pos1] {
		case '<':
			pos2 := strings.IndexByte(s[pos1:], '>')
			if pos2 == -1 {
				return "", "", s, false
			}
			before, token, after = s[:pos1], s[pos1:pos1+pos2+1], s[pos1+pos2+1:]

			if !strings.HasPrefix(token, "</") {
				// try to extract full tag with content, e.g. `<<span>content</span>>`, to optimize diff results
				if fullTokenRemaining, fullTokenAfter, ok := extractDiffTokenRemainingFullTag(after); ok {
					return before, "<" + token + fullTokenRemaining + ">", fullTokenAfter, true
				}
			}
			return before, token, after, true
		case '&':
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

	diffCodeAddedOpen   rune
	diffCodeRemovedOpen rune
	diffCodeClose       rune
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

func (hcd *highlightCodeDiff) diffEqualPartIsSpaceOnly(s string) bool {
	for _, r := range s {
		if r >= hcd.placeholderBegin {
			recovered := hcd.placeholderTokenMap[r]
			if strings.HasPrefix(recovered, "<<") {
				return false // a full tag with content, it can't be space-only
			} else if strings.HasPrefix(recovered, "<") {
				continue // a single opening/closing tag, skip the tag and continue to check the content
			}
			return false // otherwise, it must be an HTML entity, it can't be space-only
		}
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if !isSpace {
			return false
		}
	}
	return true
}

func (hcd *highlightCodeDiff) diffLineWithHighlight(lineType DiffLineType, codeA, codeB template.HTML) template.HTML {
	hcd.collectUsedRunes(codeA)
	hcd.collectUsedRunes(codeB)

	convertedCodeA := hcd.convertToPlaceholders(codeA)
	convertedCodeB := hcd.convertToPlaceholders(codeB)

	dmp := defaultDiffMatchPatch()
	diffs := dmp.DiffMain(convertedCodeA, convertedCodeB, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	buf := bytes.NewBuffer(nil)

	if hcd.diffCodeClose == 0 {
		// tests can pre-set the placeholders
		hcd.diffCodeAddedOpen = hcd.registerTokenAsPlaceholder(`<span class="added-code">`)
		hcd.diffCodeRemovedOpen = hcd.registerTokenAsPlaceholder(`<span class="removed-code">`)
		hcd.diffCodeClose = hcd.registerTokenAsPlaceholder(`</span><!-- diff-code-close -->`)
	}

	equalPartSpaceOnly := true
	for _, diff := range diffs {
		if diff.Type != diffmatchpatch.DiffEqual {
			continue
		}
		if equalPartSpaceOnly = hcd.diffEqualPartIsSpaceOnly(diff.Text); !equalPartSpaceOnly {
			break
		}
	}

	// only add "added"/"removed" tags when needed:
	// * non-space contents appear in the DiffEqual parts (not a full-line add/del)
	// * placeholder map still works (not exhausted, can get the closing tag placeholder)
	addDiffTags := !equalPartSpaceOnly && hcd.diffCodeClose != 0
	if addDiffTags {
		for _, diff := range diffs {
			switch {
			case diff.Type == diffmatchpatch.DiffEqual:
				buf.WriteString(diff.Text)
			case diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd:
				buf.WriteRune(hcd.diffCodeAddedOpen)
				buf.WriteString(diff.Text)
				buf.WriteRune(hcd.diffCodeClose)
			case diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel:
				buf.WriteRune(hcd.diffCodeRemovedOpen)
				buf.WriteString(diff.Text)
				buf.WriteRune(hcd.diffCodeClose)
			}
		}
	} else {
		// the caller will still add added/removed backgrounds for the whole line
		for _, diff := range diffs {
			take := diff.Type == diffmatchpatch.DiffEqual || (diff.Type == diffmatchpatch.DiffInsert && lineType == DiffLineAdd) || (diff.Type == diffmatchpatch.DiffDelete && lineType == DiffLineDel)
			if take {
				buf.WriteString(diff.Text)
			}
		}
	}
	return hcd.recoverOneDiff(buf.String())
}

func (hcd *highlightCodeDiff) registerTokenAsPlaceholder(token string) rune {
	recovered := token
	if token[0] == '<' && token[1] != '<' {
		// when recovering a single tag, only use the tag itself, ignore the trailing comment (for how the comment is generated, see the code in `convert` function)
		recovered = token[:strings.IndexByte(token, '>')+1]
	}
	placeholder, ok := hcd.tokenPlaceholderMap[token]
	if !ok {
		placeholder = hcd.nextPlaceholder()
		if placeholder != 0 {
			hcd.tokenPlaceholderMap[token] = placeholder
			hcd.placeholderTokenMap[placeholder] = recovered
		}
	}
	return placeholder
}

// convertToPlaceholders totally depends on Chroma's valid HTML output and its structure, do not use these functions for other purposes.
func (hcd *highlightCodeDiff) convertToPlaceholders(htmlContent template.HTML) string {
	var tagStack []string
	res := strings.Builder{}

	htmlCode := string(htmlContent)
	var beforeToken, token string
	var valid bool
	for {
		beforeToken, token, htmlCode, valid = extractDiffToken(htmlCode)
		if !valid || token == "" {
			break
		}
		// write the content before the token into result string, and consume the token in the string
		res.WriteString(beforeToken)

		// the standard chroma highlight HTML is `<span class="line [hl]"><span class="cl"> ... </span></span>`
		// the line wrapper tags should be removed before diff
		if strings.HasPrefix(token, `<span class="line`) || strings.HasPrefix(token, `<span class="cl"`) {
			continue
		}

		var tokenInMap string
		if strings.HasPrefix(token, "</") { // for closing tag
			if len(tagStack) == 0 {
				continue // no opening tag but see closing tag, skip it
			}
			// make sure the closing tag in map is related to the open tag, to make the diff algorithm can match the opening/closing tags
			// the closing tag will be recorded in the map by key "</span><!-- <span the-opening> -->" for "<span the-opening>"
			tokenInMap = token + "<!-- " + tagStack[len(tagStack)-1] + "-->"
			tagStack = tagStack[:len(tagStack)-1]
		} else if token[0] == '<' {
			if token[1] == '<' {
				// full tag `<<span>content</span>>`, recover to `<span>content</span>`
				tokenInMap = token
			} else {
				// opening tag
				tokenInMap = token
				tagStack = append(tagStack, token)
			}
		} else if token[0] == '&' { // for HTML entity
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
			if strings.HasPrefix(token, "<<") {
				pos1 := strings.IndexByte(token, '>')
				pos2 := strings.LastIndexByte(token, '<')
				res.WriteString(token[pos1+1 : pos2]) // recover to `content` from "<<span>content</span>>"
			}
			if strings.HasPrefix(token, "&") {
				// when the token is an HTML entity, something must be outputted even if there is no placeholder.
				res.WriteRune(0xFFFD)      // replacement character TODO: how to handle this case more gracefully?
				res.WriteString(token[1:]) // still output the entity code part, otherwise there will be no diff result.
			}
		}
	}

	// write the remaining string
	res.WriteString(htmlCode)
	return res.String()
}

// recoverOneRune tries to recover one rune
// * if the rune is a placeholder, it will be recovered to the corresponding content
// * otherwise it will be returned as is
func (hcd *highlightCodeDiff) recoverOneRune(buf []byte) (r rune, runeLen int, isSingleTag bool, recovered string) {
	r, runeLen = utf8.DecodeRune(buf)
	token := hcd.placeholderTokenMap[r]
	if token == "" {
		return r, runeLen, false, "" // rune itself, not a placeholder
	} else if token[0] == '<' {
		if token[1] == '<' {
			return 0, runeLen, false, token[1 : len(token)-1] // full tag `<<span>content</span>>`, recover to `<span>content</span>`
		}
		return r, runeLen, true, token // single tag
	}
	return 0, runeLen, false, token // HTML entity
}

func (hcd *highlightCodeDiff) recoverOneDiff(str string) template.HTML {
	sb := strings.Builder{}
	var tagStack []string
	var diffCodeOpenTag string
	diffCodeCloseTag := hcd.placeholderTokenMap[hcd.diffCodeClose]
	strBytes := util.UnsafeStringToBytes(str)

	// this loop is slightly longer than expected, for performance consideration
	for idx := 0; idx < len(strBytes); {
		// take a look at the next rune
		r, runeLen, isSingleTag, recovered := hcd.recoverOneRune(strBytes[idx:])
		idx += runeLen

		// loop section 1: if it isn't a single tag, then try to find the following runes until the next single tag, and recover them together
		if !isSingleTag {
			if diffCodeOpenTag != "" {
				// start the "added/removed diff tag" if the current token is in the diff part
				sb.WriteString(diffCodeOpenTag)
			}
			if recovered != "" {
				sb.WriteString(recovered)
			} else {
				sb.WriteRune(r)
			}
			// inner loop to recover following runes until the next single tag
			for idx < len(strBytes) {
				r, runeLen, isSingleTag, recovered = hcd.recoverOneRune(strBytes[idx:])
				idx += runeLen
				if isSingleTag {
					break
				}
				if recovered != "" {
					sb.WriteString(recovered)
				} else {
					sb.WriteRune(r)
				}
			}
			if diffCodeOpenTag != "" {
				// end the "added/removed diff tag" if the current token is in the diff part
				sb.WriteString(diffCodeCloseTag)
			}
		}

		if !isSingleTag {
			break // the inner loop has already consumed all remaining runes, no more single tag found
		}

		// loop section 2: for opening/closing HTML tags
		placeholder := r
		if recovered[1] != '/' { // opening tag
			if placeholder == hcd.diffCodeAddedOpen || placeholder == hcd.diffCodeRemovedOpen {
				diffCodeOpenTag = recovered
				recovered = ""
			} else {
				tagStack = append(tagStack, recovered)
			}
		} else { // closing tag
			if placeholder == hcd.diffCodeClose {
				diffCodeOpenTag = "" // the highlighted diff is closed, no more diff
				recovered = ""
			} else if len(tagStack) != 0 {
				tagStack = tagStack[:len(tagStack)-1]
			} else {
				recovered = ""
			}
		}
		sb.WriteString(recovered)
	}

	// close all opening tags
	for i := len(tagStack) - 1; i >= 0; i-- {
		tagToClose := tagStack[i]
		// get the closing tag "</span>" from "<span class=...>" or "<span>"
		pos := strings.IndexAny(tagToClose, " >")
		// pos must be positive, because the tags were pushed by us
		sb.WriteString("</" + tagToClose[1:pos] + ">")
	}
	return template.HTML(sb.String())
}
