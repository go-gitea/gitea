// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"golang.org/x/net/html"
)

// Parser defines an interface for parsering markup file to HTML
type Parser interface {
	Name() string // markup format name
	Extensions() []string
	Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte
}

var (
	parsers = make(map[string]Parser)
)

// RegisterParser registers a new markup file parser
func RegisterParser(parser Parser) {
	for _, ext := range parser.Extensions() {
		parsers[strings.ToLower(ext)] = parser
	}
}

// GetParser get parser by filename
func GetParser(filename string) Parser {
	extension := strings.ToLower(filepath.Ext(filename))
	return parsers[extension]
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return render(filename, rawBytes, urlPrefix, metas, false)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(filename string, raw, urlPrefix string, metas map[string]string) string {
	return string(render(filename, []byte(raw), urlPrefix, metas, false))
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return string(render(filename, rawBytes, urlPrefix, metas, true))
}

// RenderSpecialLink renders mentions, indexes and SHA1 strings to corresponding links.
func RenderSpecialLink(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	ms := MentionPattern.FindAll(rawBytes, -1)
	for _, m := range ms {
		m = m[bytes.Index(m, []byte("@")):]
		rawBytes = bytes.Replace(rawBytes, m,
			[]byte(fmt.Sprintf(`<a href="%s">%s</a>`, URLJoin(setting.AppURL, string(m[1:])), m)), -1)
	}

	rawBytes = RenderShortLinks(rawBytes, urlPrefix, false, isWiki)
	rawBytes = RenderIssueIndexPattern(rawBytes, urlPrefix, metas)
	rawBytes = RenderCrossReferenceIssueIndexPattern(rawBytes, urlPrefix, metas)
	rawBytes = renderFullSha1Pattern(rawBytes, urlPrefix)
	rawBytes = renderSha1CurrentPattern(rawBytes, urlPrefix)
	rawBytes = renderFullIssuePattern(rawBytes, urlPrefix)
	return rawBytes
}

// PostProcess treats different types of HTML differently,
// and only renders special links for plain text blocks.
func PostProcess(rawHTML []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	startTags := make([]string, 0, 5)
	var buf bytes.Buffer
	tokenizer := html.NewTokenizer(bytes.NewReader(rawHTML))

OUTER_LOOP:
	for html.ErrorToken != tokenizer.Next() {
		token := tokenizer.Token()
		switch token.Type {
		case html.TextToken:
			buf.Write(RenderSpecialLink([]byte(token.String()), urlPrefix, metas, isWiki))

		case html.StartTagToken:
			buf.WriteString(token.String())
			tagName := token.Data
			// If this is an excluded tag, we skip processing all output until a close tag is encountered.
			if strings.EqualFold("a", tagName) || strings.EqualFold("code", tagName) || strings.EqualFold("pre", tagName) {
				stackNum := 1
				for html.ErrorToken != tokenizer.Next() {
					token = tokenizer.Token()

					// Copy the token to the output verbatim
					buf.Write(RenderShortLinks([]byte(token.String()), urlPrefix, true, isWiki))

					if token.Type == html.StartTagToken {
						if !com.IsSliceContainsStr(noEndTags, token.Data) {
							stackNum++
						}
					}

					// If this is the close tag to the outer-most, we are done
					if token.Type == html.EndTagToken {
						stackNum--

						if stackNum <= 0 && strings.EqualFold(tagName, token.Data) {
							break
						}
					}
				}
				continue OUTER_LOOP
			}

			if !com.IsSliceContainsStr(noEndTags, token.Data) {
				startTags = append(startTags, token.Data)
			}

		case html.EndTagToken:
			if len(startTags) == 0 {
				buf.WriteString(token.String())
				break
			}

			buf.Write(leftAngleBracket)
			buf.WriteString(startTags[len(startTags)-1])
			buf.Write(rightAngleBracket)
			startTags = startTags[:len(startTags)-1]
		default:
			buf.WriteString(token.String())
		}
	}

	if io.EOF == tokenizer.Err() {
		return buf.Bytes()
	}

	// If we are not at the end of the input, then some other parsing error has occurred,
	// so return the input verbatim.
	return rawHTML
}

func render(filename string, rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	extension := strings.ToLower(filepath.Ext(filename))
	if parser, ok := parsers[extension]; ok {
		urlPrefix = strings.Replace(urlPrefix, " ", "+", -1)
		result := parser.Render(rawBytes, urlPrefix, metas, isWiki)
		result = PostProcess(result, urlPrefix, metas, isWiki)
		return SanitizeBytes(result)
	}
	return nil
}

// Type returns if markup format via the filename
func Type(filename string) string {
	if parser := GetParser(filename); parser != nil {
		return parser.Name()
	}
	return ""
}

// IsMarkupFile reports whether file is a markup type file
func IsMarkupFile(name, markup string) bool {
	if parser := GetParser(name); parser != nil {
		return parser.Name() == markup
	}
	return false
}

// IsReadmeFile reports whether name looks like a README file
// based on its name.
func IsReadmeFile(name string) bool {
	name = strings.ToLower(name)
	if len(name) < 6 {
		return false
	} else if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}
