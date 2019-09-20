// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/russross/blackfriday"
)

// MarkdownStripper extends blackfriday.Renderer
type MarkdownStripper struct {
	blackfriday.Renderer
	links []string
}

const (
	blackfridayExtensions = 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK |
		blackfriday.EXTENSION_DEFINITION_LISTS |
		blackfriday.EXTENSION_FOOTNOTES |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_AUTO_HEADER_IDS |
		// Not included in modules/markup/markdown/markdown.go;
		// required here to process inline links
		blackfriday.EXTENSION_AUTOLINK
)

var (
	// validNamePattern performs only the most basic validation for user or repository names
	// Repository name should contain only alphanumeric, dash ('-'), underscore ('_') and dot ('.') characters.
	validNamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)
)

// StripMarkdown parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdown(rawBytes []byte) (string, []string) {
	stripper := &MarkdownStripper{
		links: make([]string, 0, 10),
	}
	body := blackfriday.Markdown(rawBytes, stripper, blackfridayExtensions)
	return string(body), stripper.GetLinks()
}

// block-level callbacks

// BlockCode dummy function to proceed with rendering
func (r *MarkdownStripper) BlockCode(out *bytes.Buffer, text []byte, infoString string) {
	// Not rendered
}

// BlockQuote dummy function to proceed with rendering
func (r *MarkdownStripper) BlockQuote(out *bytes.Buffer, text []byte) {
	// FIXME: perhaps it's better to leave out block quote for this?
	r.processString(out, text)
}

// BlockHtml dummy function to proceed with rendering
func (r *MarkdownStripper) BlockHtml(out *bytes.Buffer, text []byte) { //nolint
	// Not rendered
}

// Header dummy function to proceed with rendering
func (r *MarkdownStripper) Header(out *bytes.Buffer, text func() bool, level int, id string) {
	text()
}

// HRule dummy function to proceed with rendering
func (r *MarkdownStripper) HRule(out *bytes.Buffer) {
	// Not rendered
}

// List dummy function to proceed with rendering
func (r *MarkdownStripper) List(out *bytes.Buffer, text func() bool, flags int) {
	text()
}

// ListItem dummy function to proceed with rendering
func (r *MarkdownStripper) ListItem(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text)
}

// Paragraph dummy function to proceed with rendering
func (r *MarkdownStripper) Paragraph(out *bytes.Buffer, text func() bool) {
	text()
}

// Table dummy function to proceed with rendering
func (r *MarkdownStripper) Table(out *bytes.Buffer, header []byte, body []byte, columnData []int) {
	r.processString(out, header)
	r.processString(out, body)
}

// TableRow dummy function to proceed with rendering
func (r *MarkdownStripper) TableRow(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// TableHeaderCell dummy function to proceed with rendering
func (r *MarkdownStripper) TableHeaderCell(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text)
}

// TableCell dummy function to proceed with rendering
func (r *MarkdownStripper) TableCell(out *bytes.Buffer, text []byte, flags int) {
	r.processString(out, text)
}

// Footnotes dummy function to proceed with rendering
func (r *MarkdownStripper) Footnotes(out *bytes.Buffer, text func() bool) {
	text()
}

// FootnoteItem dummy function to proceed with rendering
func (r *MarkdownStripper) FootnoteItem(out *bytes.Buffer, name, text []byte, flags int) {
	r.processString(out, text)
}

// TitleBlock dummy function to proceed with rendering
func (r *MarkdownStripper) TitleBlock(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// Span-level callbacks

// AutoLink dummy function to proceed with rendering
func (r *MarkdownStripper) AutoLink(out *bytes.Buffer, link []byte, kind int) {
	r.processLink(out, link, []byte{})
}

// CodeSpan dummy function to proceed with rendering
func (r *MarkdownStripper) CodeSpan(out *bytes.Buffer, text []byte) {
	// Not rendered
}

// DoubleEmphasis dummy function to proceed with rendering
func (r *MarkdownStripper) DoubleEmphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// Emphasis dummy function to proceed with rendering
func (r *MarkdownStripper) Emphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// Image dummy function to proceed with rendering
func (r *MarkdownStripper) Image(out *bytes.Buffer, link []byte, title []byte, alt []byte) {
	// Not rendered
}

// LineBreak dummy function to proceed with rendering
func (r *MarkdownStripper) LineBreak(out *bytes.Buffer) {
	// Not rendered
}

// Link dummy function to proceed with rendering
func (r *MarkdownStripper) Link(out *bytes.Buffer, link []byte, title []byte, content []byte) {
	r.processLink(out, link, content)
}

// RawHtmlTag dummy function to proceed with rendering
func (r *MarkdownStripper) RawHtmlTag(out *bytes.Buffer, tag []byte) { //nolint
	// Not rendered
}

// TripleEmphasis dummy function to proceed with rendering
func (r *MarkdownStripper) TripleEmphasis(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// StrikeThrough dummy function to proceed with rendering
func (r *MarkdownStripper) StrikeThrough(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// FootnoteRef dummy function to proceed with rendering
func (r *MarkdownStripper) FootnoteRef(out *bytes.Buffer, ref []byte, id int) {
	// Not rendered
}

// Low-level callbacks

// Entity dummy function to proceed with rendering
func (r *MarkdownStripper) Entity(out *bytes.Buffer, entity []byte) {
	// FIXME: literal entities are not parsed; perhaps they should
}

// NormalText dummy function to proceed with rendering
func (r *MarkdownStripper) NormalText(out *bytes.Buffer, text []byte) {
	r.processString(out, text)
}

// Header and footer

// DocumentHeader dummy function to proceed with rendering
func (r *MarkdownStripper) DocumentHeader(out *bytes.Buffer) {
}

// DocumentFooter dummy function to proceed with rendering
func (r *MarkdownStripper) DocumentFooter(out *bytes.Buffer) {
}

// GetFlags returns rendering flags
func (r *MarkdownStripper) GetFlags() int {
	return 0
}

func doubleSpace(out *bytes.Buffer) {
	if out.Len() > 0 {
		out.WriteByte('\n')
	}
}

func (r *MarkdownStripper) processString(out *bytes.Buffer, text []byte) {
	// Always break-up words
	doubleSpace(out)
	out.Write(text)
}
func (r *MarkdownStripper) processLink(out *bytes.Buffer, link []byte, content []byte) {
	// Links are processed out of band
	r.links = append(r.links, string(link))
}

// GetLinks returns the list of link data collected while parsing
func (r *MarkdownStripper) GetLinks() []string {
	return r.links
}

// FindAllMentions matches mention patterns in given content
// and returns a list of found unvalidated user names without @ prefix.
func FindAllMentions(content string) []string {
	content, _ = StripMarkdown([]byte(content))
	mentions := mentionPattern.FindAllStringSubmatch(content, -1)
	ret := make([]string, len(mentions))
	for i, val := range mentions {
		ret[i] = val[1][1:]
	}
	return ret
}

type RawIssueReference struct {
	Index int64
	Owner string
	Name  string
}

// FindAllIssueReferences matches issue reference patterns in given content
// and returns a list of unvalidated references.
func FindAllIssueReferences(content string) []*RawIssueReference {

	content, links := StripMarkdown([]byte(content))
	ret := make([]*RawIssueReference, 0, 10)

	matches := issueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(match[1], false); ref != nil {
			ret = append(ret, ref)
		}
	}

	matches = crossReferenceIssueNumericPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if ref := getCrossReference(match[1], false); ref != nil {
			ret = append(ret, ref)
		}
	}

	var giteahost string
	if uapp, err := url.Parse(setting.AppURL); err == nil {
		giteahost = strings.ToLower(uapp.Host)
	}

	for _, link := range links {
		if u, err := url.Parse(link); err == nil {
			// Note: we're not attempting to match the URL scheme (http/https)
			host := strings.ToLower(u.Host)
			if host != "" && host != giteahost {
				continue
			}
			if u.EscapedPath() == "" || u.EscapedPath()[0] != '/' {
				continue
			}
			parts := strings.Split(u.EscapedPath()[1:], "/")
			// user/repo/issues/3
			if len(parts) != 4 {
				continue
			}
			if parts[2] != "issues" && parts[2] != "pulls" {
				continue
			}
			if ref := getCrossReference(parts[0]+"/"+parts[1]+"#"+parts[3], true); ref != nil {
				ret = append(ret, ref)
			}
		}
	}
	return ret
}

func getCrossReference(s string, fromLink bool) *RawIssueReference {
	parts := strings.Split(s, "#")
	if len(parts) != 2 {
		return nil
	}
	repo, issue := parts[0], parts[1]
	index, err := strconv.ParseInt(issue, 10, 64)
	if err != nil {
		return nil
	}
	if repo == "" {
		if fromLink {
			// Markdown links must specify owner/repo
			return nil
		}
		return &RawIssueReference{Index: index}
	}
	parts = strings.Split(strings.ToLower(repo), "/")
	if len(parts) != 2 {
		return nil
	}
	owner, name := parts[0], parts[1]
	if !validNamePattern.MatchString(owner) || !validNamePattern.MatchString(name) {
		return nil
	}
	return &RawIssueReference{Index: index, Owner: owner, Name: name}
}
