// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"github.com/russross/blackfriday"
	"golang.org/x/net/html"
)

// Issue name styles
const (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
)

// IsMarkdownFile reports whether name looks like a Markdown file
// based on its extension.
func IsMarkdownFile(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	for _, ext := range setting.Markdown.FileExtensions {
		if strings.ToLower(ext) == extension {
			return true
		}
	}
	return false
}

var (
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the user does not exist, the issue does not exist, the commit does not exist, etc.
	// While fast, this is also incorrect and lead to false positives.

	// MentionPattern matches string that mentions someone, e.g. @Unknwon
	MentionPattern = regexp.MustCompile(`(\s|^|\W)@[0-9a-zA-Z-_\.]+`)

	// IssueNumericPattern matches string that references to a numeric issue, e.g. #1287
	IssueNumericPattern = regexp.MustCompile(`( |^|\()#[0-9]+\b`)
	// IssueAlphanumericPattern matches string that references to an alphanumeric issue, e.g. ABC-1234
	IssueAlphanumericPattern = regexp.MustCompile(`( |^|\()[A-Z]{1,10}-[1-9][0-9]*\b`)
	// CrossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	CrossReferenceIssueNumericPattern = regexp.MustCompile(`( |^)[0-9a-zA-Z]+/[0-9a-zA-Z]+#[0-9]+\b`)

	// Sha1CurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, the regex matches the hash from 7 to 40 chars in length
	// so that abbreviated hash links can be used as well. This matches git and github useability.
	Sha1CurrentPattern = regexp.MustCompile(`(?:^|\s|\()([0-9a-f]{7,40})\b`)

	// ShortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	ShortLinkPattern = regexp.MustCompile(`(\[\[.*\]\]\w*)`)

	// AnySHA1Pattern allows to split url containing SHA into parts
	AnySHA1Pattern = regexp.MustCompile(`(http\S*)://(\S+)/(\S+)/(\S+)/(\S+)/([0-9a-f]{40})(?:/?([^#\s]+)?(?:#(\S+))?)?`)

	// IssueFullPattern allows to split issue (and pull) URLs into parts
	IssueFullPattern = regexp.MustCompile(`(?:^|\s|\()(http\S*)://((?:[^\s/]+/)+)((?:\w{1,10}-)?[1-9][0-9]*)([\?|#]\S+.(\S+)?)?\b`)

	validLinksPattern = regexp.MustCompile(`^[a-z][\w-]+://`)
)

// isLink reports whether link fits valid format.
func isLink(link []byte) bool {
	return validLinksPattern.Match(link)
}

// FindAllMentions matches mention patterns in given content
// and returns a list of found user names without @ prefix.
func FindAllMentions(content string) []string {
	mentions := MentionPattern.FindAllString(content, -1)
	for i := range mentions {
		mentions[i] = mentions[i][strings.Index(mentions[i], "@")+1:] // Strip @ character
	}
	return mentions
}

// Renderer is a extended version of underlying render object.
type Renderer struct {
	blackfriday.Renderer
	urlPrefix      string
	isWikiMarkdown bool
}

// Link defines how formal links should be processed to produce corresponding HTML elements.
func (r *Renderer) Link(out *bytes.Buffer, link []byte, title []byte, content []byte) {
	if len(link) > 0 && !isLink(link) {
		if link[0] != '#' {
			lnk := string(link)
			if r.isWikiMarkdown {
				lnk = URLJoin("wiki", lnk)
			}
			mLink := URLJoin(r.urlPrefix, lnk)
			link = []byte(mLink)
		}
	}

	r.Renderer.Link(out, link, title, content)
}

// List renders markdown bullet or digit lists to HTML
func (r *Renderer) List(out *bytes.Buffer, text func() bool, flags int) {
	marker := out.Len()
	if out.Len() > 0 {
		out.WriteByte('\n')
	}

	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("<dl>")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("<ol class='ui list'>")
	} else {
		out.WriteString("<ul class='ui list'>")
	}
	if !text() {
		out.Truncate(marker)
		return
	}
	if flags&blackfriday.LIST_TYPE_DEFINITION != 0 {
		out.WriteString("</dl>\n")
	} else if flags&blackfriday.LIST_TYPE_ORDERED != 0 {
		out.WriteString("</ol>\n")
	} else {
		out.WriteString("</ul>\n")
	}
}

// ListItem defines how list items should be processed to produce corresponding HTML elements.
func (r *Renderer) ListItem(out *bytes.Buffer, text []byte, flags int) {
	// Detect procedures to draw checkboxes.
	prefix := ""
	if bytes.HasPrefix(text, []byte("<p>")) {
		prefix = "<p>"
	}
	switch {
	case bytes.HasPrefix(text, []byte(prefix+"[ ] ")):
		text = append([]byte(`<span class="ui fitted disabled checkbox"><input type="checkbox" disabled="disabled" /><label /></span>`), text[3+len(prefix):]...)
		if prefix != "" {
			text = bytes.Replace(text, []byte(prefix), []byte{}, 1)
		}
	case bytes.HasPrefix(text, []byte(prefix+"[x] ")):
		text = append([]byte(`<span class="ui checked fitted disabled checkbox"><input type="checkbox" checked="" disabled="disabled" /><label /></span>`), text[3+len(prefix):]...)
		if prefix != "" {
			text = bytes.Replace(text, []byte(prefix), []byte{}, 1)
		}
	}
	r.Renderer.ListItem(out, text, flags)
}

// Note: this section is for purpose of increase performance and
// reduce memory allocation at runtime since they are constant literals.
var (
	svgSuffix         = []byte(".svg")
	svgSuffixWithMark = []byte(".svg?")
)

// Image defines how images should be processed to produce corresponding HTML elements.
func (r *Renderer) Image(out *bytes.Buffer, link []byte, title []byte, alt []byte) {
	prefix := r.urlPrefix
	if r.isWikiMarkdown {
		prefix = URLJoin(prefix, "wiki", "src")
	}
	prefix = strings.Replace(prefix, "/src/", "/raw/", 1)
	if len(link) > 0 {
		if isLink(link) {
			// External link with .svg suffix usually means CI status.
			// TODO: define a keyword to allow non-svg images render as external link.
			if bytes.HasSuffix(link, svgSuffix) || bytes.Contains(link, svgSuffixWithMark) {
				r.Renderer.Image(out, link, title, alt)
				return
			}
		} else {
			lnk := string(link)
			lnk = URLJoin(prefix, lnk)
			lnk = strings.Replace(lnk, " ", "+", -1)
			link = []byte(lnk)
		}
	}

	out.WriteString(`<a href="`)
	out.Write(link)
	out.WriteString(`">`)
	r.Renderer.Image(out, link, title, alt)
	out.WriteString("</a>")
}

// cutoutVerbosePrefix cutouts URL prefix including sub-path to
// return a clean unified string of request URL path.
func cutoutVerbosePrefix(prefix string) string {
	if len(prefix) == 0 || prefix[0] != '/' {
		return prefix
	}
	count := 0
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '/' {
			count++
		}
		if count >= 3+setting.AppSubURLDepth {
			return prefix[:i]
		}
	}
	return prefix
}

// URLJoin joins url components, like path.Join, but preserving contents
func URLJoin(base string, elems ...string) string {
	u, err := url.Parse(base)
	if err != nil {
		log.Error(4, "URLJoin: Invalid base URL %s", base)
		return ""
	}
	joinArgs := make([]string, 0, len(elems)+1)
	joinArgs = append(joinArgs, u.Path)
	joinArgs = append(joinArgs, elems...)
	u.Path = path.Join(joinArgs...)
	return u.String()
}

// RenderIssueIndexPattern renders issue indexes to corresponding links.
func RenderIssueIndexPattern(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	urlPrefix = cutoutVerbosePrefix(urlPrefix)

	pattern := IssueNumericPattern
	if metas["style"] == IssueNameStyleAlphanumeric {
		pattern = IssueAlphanumericPattern
	}

	ms := pattern.FindAll(rawBytes, -1)
	for _, m := range ms {
		if m[0] == ' ' || m[0] == '(' {
			m = m[1:] // ignore leading space or opening parentheses
		}
		var link string
		if metas == nil {
			link = fmt.Sprintf(`<a href="%s">%s</a>`, URLJoin(urlPrefix, "issues", string(m[1:])), m)
		} else {
			// Support for external issue tracker
			if metas["style"] == IssueNameStyleAlphanumeric {
				metas["index"] = string(m)
			} else {
				metas["index"] = string(m[1:])
			}
			link = fmt.Sprintf(`<a href="%s">%s</a>`, com.Expand(metas["format"], metas), m)
		}
		rawBytes = bytes.Replace(rawBytes, m, []byte(link), 1)
	}
	return rawBytes
}

// IsSameDomain checks if given url string has the same hostname as current Gitea instance
func IsSameDomain(s string) bool {
	if strings.HasPrefix(s, "/") {
		return true
	}
	if uapp, err := url.Parse(setting.AppURL); err == nil {
		if u, err := url.Parse(s); err == nil {
			return u.Host == uapp.Host
		}
		return false
	}
	return false
}

// renderFullSha1Pattern renders SHA containing URLs
func renderFullSha1Pattern(rawBytes []byte, urlPrefix string) []byte {
	ms := AnySHA1Pattern.FindAllSubmatch(rawBytes, -1)
	for _, m := range ms {
		all := m[0]
		protocol := string(m[1])
		paths := string(m[2])
		path := protocol + "://" + paths
		author := string(m[3])
		repoName := string(m[4])
		path = URLJoin(path, author, repoName)
		ltype := "src"
		itemType := m[5]
		if IsSameDomain(paths) {
			ltype = string(itemType)
		} else if string(itemType) == "commit" {
			ltype = "commit"
		}
		sha := m[6]
		var subtree string
		if len(m) > 7 && len(m[7]) > 0 {
			subtree = string(m[7])
		}
		var line []byte
		if len(m) > 8 && len(m[8]) > 0 {
			line = m[8]
		}
		urlSuffix := ""
		text := base.ShortSha(string(sha))
		if subtree != "" {
			urlSuffix = "/" + subtree
			text += urlSuffix
		}
		if line != nil {
			value := string(line)
			urlSuffix += "#"
			urlSuffix += value
			text += " ("
			text += value
			text += ")"
		}
		rawBytes = bytes.Replace(rawBytes, all, []byte(fmt.Sprintf(
			`<a href="%s">%s</a>`, URLJoin(path, ltype, string(sha))+urlSuffix, text)), -1)
	}
	return rawBytes
}

// renderFullIssuePattern renders issues-like URLs
func renderFullIssuePattern(rawBytes []byte, urlPrefix string) []byte {
	ms := IssueFullPattern.FindAllSubmatch(rawBytes, -1)
	for _, m := range ms {
		all := m[0]
		protocol := string(m[1])
		paths := bytes.Split(m[2], []byte("/"))
		paths = paths[:len(paths)-1]
		if bytes.HasPrefix(paths[0], []byte("gist.")) {
			continue
		}
		path := protocol + "://" + string(m[2])
		id := string(m[3])
		path = URLJoin(path, id)
		var comment []byte
		if len(m) > 3 {
			comment = m[4]
		}
		urlSuffix := ""
		text := "#" + id
		if comment != nil {
			urlSuffix += string(comment)
			text += " <i class='comment icon'></i>"
		}
		rawBytes = bytes.Replace(rawBytes, all, []byte(fmt.Sprintf(
			`<a href="%s%s">%s</a>`, path, urlSuffix, text)), -1)
	}
	return rawBytes
}

func firstIndexOfByte(sl []byte, target byte) int {
	for i := 0; i < len(sl); i++ {
		if sl[i] == target {
			return i
		}
	}
	return -1
}

func lastIndexOfByte(sl []byte, target byte) int {
	for i := len(sl) - 1; i >= 0; i-- {
		if sl[i] == target {
			return i
		}
	}
	return -1
}

// RenderShortLinks processes [[syntax]]
//
// noLink flag disables making link tags when set to true
// so this function just replaces the whole [[...]] with the content text
//
// isWikiMarkdown is a flag to choose linking url prefix
func RenderShortLinks(rawBytes []byte, urlPrefix string, noLink bool, isWikiMarkdown bool) []byte {
	ms := ShortLinkPattern.FindAll(rawBytes, -1)
	for _, m := range ms {
		orig := bytes.TrimSpace(m)
		m = orig[2:]
		tailPos := lastIndexOfByte(m, ']') + 1
		tail := []byte{}
		if tailPos < len(m) {
			tail = m[tailPos:]
			m = m[:tailPos-1]
		}
		m = m[:len(m)-2]
		props := map[string]string{}

		// MediaWiki uses [[link|text]], while GitHub uses [[text|link]]
		// It makes page handling terrible, but we prefer GitHub syntax
		// And fall back to MediaWiki only when it is obvious from the look
		// Of text and link contents
		sl := bytes.Split(m, []byte("|"))
		for _, v := range sl {
			switch bytes.Count(v, []byte("=")) {

			// Piped args without = sign, these are mandatory arguments
			case 0:
				{
					sv := string(v)
					if props["name"] == "" {
						if isLink(v) {
							// If we clearly see it is a link, we save it so

							// But first we need to ensure, that if both mandatory args provided
							// look like links, we stick to GitHub syntax
							if props["link"] != "" {
								props["name"] = props["link"]
							}

							props["link"] = strings.TrimSpace(sv)
						} else {
							props["name"] = sv
						}
					} else {
						props["link"] = strings.TrimSpace(sv)
					}
				}

			// Piped args with = sign, these are optional arguments
			case 1:
				{
					sep := firstIndexOfByte(v, '=')
					key, val := string(v[:sep]), html.UnescapeString(string(v[sep+1:]))
					lastCharIndex := len(val) - 1
					if (val[0] == '"' || val[0] == '\'') && (val[lastCharIndex] == '"' || val[lastCharIndex] == '\'') {
						val = val[1:lastCharIndex]
					}
					props[key] = val
				}
			}
		}

		var name string
		var link string
		if props["link"] != "" {
			link = props["link"]
		} else if props["name"] != "" {
			link = props["name"]
		}
		if props["title"] != "" {
			name = props["title"]
		} else if props["name"] != "" {
			name = props["name"]
		} else {
			name = link
		}

		name += string(tail)
		image := false
		ext := filepath.Ext(string(link))
		if ext != "" {
			switch ext {
			case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".gif", ".bmp", ".ico", ".svg":
				{
					image = true
				}
			}
		}
		absoluteLink := isLink([]byte(link))
		if !absoluteLink {
			link = strings.Replace(link, " ", "+", -1)
		}
		if image {
			if !absoluteLink {
				if IsSameDomain(urlPrefix) {
					urlPrefix = strings.Replace(urlPrefix, "/src/", "/raw/", 1)
				}
				if isWikiMarkdown {
					link = URLJoin("wiki", "raw", link)
				}
				link = URLJoin(urlPrefix, link)
			}
			title := props["title"]
			if title == "" {
				title = props["alt"]
			}
			if title == "" {
				title = path.Base(string(name))
			}
			alt := props["alt"]
			if alt == "" {
				alt = name
			}
			if alt != "" {
				alt = `alt="` + alt + `"`
			}
			name = fmt.Sprintf(`<img src="%s" %s title="%s" />`, link, alt, title)
		} else if !absoluteLink {
			if isWikiMarkdown {
				link = URLJoin("wiki", link)
			}
			link = URLJoin(urlPrefix, link)
		}
		if noLink {
			rawBytes = bytes.Replace(rawBytes, orig, []byte(name), -1)
		} else {
			rawBytes = bytes.Replace(rawBytes, orig,
				[]byte(fmt.Sprintf(`<a href="%s">%s</a>`, link, name)), -1)
		}
	}
	return rawBytes
}

// RenderCrossReferenceIssueIndexPattern renders issue indexes from other repositories to corresponding links.
func RenderCrossReferenceIssueIndexPattern(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	ms := CrossReferenceIssueNumericPattern.FindAll(rawBytes, -1)
	for _, m := range ms {
		if m[0] == ' ' || m[0] == '(' {
			m = m[1:] // ignore leading space or opening parentheses
		}

		repo := string(bytes.Split(m, []byte("#"))[0])
		issue := string(bytes.Split(m, []byte("#"))[1])

		link := fmt.Sprintf(`<a href="%s">%s</a>`, URLJoin(setting.AppURL, repo, "issues", issue), m)
		rawBytes = bytes.Replace(rawBytes, m, []byte(link), 1)
	}
	return rawBytes
}

// renderSha1CurrentPattern renders SHA1 strings to corresponding links that assumes in the same repository.
func renderSha1CurrentPattern(rawBytes []byte, urlPrefix string) []byte {
	ms := Sha1CurrentPattern.FindAllSubmatch(rawBytes, -1)
	for _, m := range ms {
		hash := m[1]
		// The regex does not lie, it matches the hash pattern.
		// However, a regex cannot know if a hash actually exists or not.
		// We could assume that a SHA1 hash should probably contain alphas AND numerics
		// but that is not always the case.
		// Although unlikely, deadbeef and 1234567 are valid short forms of SHA1 hash
		// as used by git and github for linking and thus we have to do similar.
		rawBytes = bytes.Replace(rawBytes, hash, []byte(fmt.Sprintf(
			`<a href="%s">%s</a>`, URLJoin(urlPrefix, "commit", string(hash)), base.ShortSha(string(hash)))), -1)
	}
	return rawBytes
}

// RenderSpecialLink renders mentions, indexes and SHA1 strings to corresponding links.
func RenderSpecialLink(rawBytes []byte, urlPrefix string, metas map[string]string, isWikiMarkdown bool) []byte {
	ms := MentionPattern.FindAll(rawBytes, -1)
	for _, m := range ms {
		m = m[bytes.Index(m, []byte("@")):]
		rawBytes = bytes.Replace(rawBytes, m,
			[]byte(fmt.Sprintf(`<a href="%s">%s</a>`, URLJoin(setting.AppURL, string(m[1:])), m)), -1)
	}

	rawBytes = RenderShortLinks(rawBytes, urlPrefix, false, isWikiMarkdown)
	rawBytes = RenderIssueIndexPattern(rawBytes, urlPrefix, metas)
	rawBytes = RenderCrossReferenceIssueIndexPattern(rawBytes, urlPrefix, metas)
	rawBytes = renderFullSha1Pattern(rawBytes, urlPrefix)
	rawBytes = renderSha1CurrentPattern(rawBytes, urlPrefix)
	rawBytes = renderFullIssuePattern(rawBytes, urlPrefix)
	return rawBytes
}

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	htmlFlags := 0
	htmlFlags |= blackfriday.HTML_SKIP_STYLE
	htmlFlags |= blackfriday.HTML_OMIT_CONTENTS
	renderer := &Renderer{
		Renderer:       blackfriday.HtmlRenderer(htmlFlags, "", ""),
		urlPrefix:      urlPrefix,
		isWikiMarkdown: wikiMarkdown,
	}

	// set up the parser
	extensions := 0
	extensions |= blackfriday.EXTENSION_NO_INTRA_EMPHASIS
	extensions |= blackfriday.EXTENSION_TABLES
	extensions |= blackfriday.EXTENSION_FENCED_CODE
	extensions |= blackfriday.EXTENSION_STRIKETHROUGH
	extensions |= blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK

	if setting.Markdown.EnableHardLineBreak {
		extensions |= blackfriday.EXTENSION_HARD_LINE_BREAK
	}

	body = blackfriday.Markdown(body, renderer, extensions)
	return body
}

var (
	leftAngleBracket  = []byte("</")
	rightAngleBracket = []byte(">")
)

var noEndTags = []string{"img", "input", "br", "hr"}

// PostProcess treats different types of HTML differently,
// and only renders special links for plain text blocks.
func PostProcess(rawHTML []byte, urlPrefix string, metas map[string]string, isWikiMarkdown bool) []byte {
	startTags := make([]string, 0, 5)
	var buf bytes.Buffer
	tokenizer := html.NewTokenizer(bytes.NewReader(rawHTML))

OUTER_LOOP:
	for html.ErrorToken != tokenizer.Next() {
		token := tokenizer.Token()
		switch token.Type {
		case html.TextToken:
			buf.Write(RenderSpecialLink([]byte(token.String()), urlPrefix, metas, isWikiMarkdown))

		case html.StartTagToken:
			buf.WriteString(token.String())
			tagName := token.Data
			// If this is an excluded tag, we skip processing all output until a close tag is encountered.
			if strings.EqualFold("a", tagName) || strings.EqualFold("code", tagName) || strings.EqualFold("pre", tagName) {
				stackNum := 1
				for html.ErrorToken != tokenizer.Next() {
					token = tokenizer.Token()

					// Copy the token to the output verbatim
					buf.Write(RenderShortLinks([]byte(token.String()), urlPrefix, true, isWikiMarkdown))

					if token.Type == html.StartTagToken && !com.IsSliceContainsStr(noEndTags, token.Data) {
						stackNum++
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

			if !com.IsSliceContainsStr(noEndTags, tagName) {
				startTags = append(startTags, tagName)
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

// Render renders Markdown to HTML with all specific handling stuff.
func render(rawBytes []byte, urlPrefix string, metas map[string]string, isWikiMarkdown bool) []byte {
	urlPrefix = strings.Replace(urlPrefix, " ", "+", -1)
	result := RenderRaw(rawBytes, urlPrefix, isWikiMarkdown)
	result = PostProcess(result, urlPrefix, metas, isWikiMarkdown)
	result = SanitizeBytes(result)
	return result
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return render(rawBytes, urlPrefix, metas, false)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(raw, urlPrefix string, metas map[string]string) string {
	return string(render([]byte(raw), urlPrefix, metas, false))
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return string(render(rawBytes, urlPrefix, metas, true))
}

var (
	// MarkupName describes markup's name
	MarkupName = "markdown"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser
type Parser struct {
}

// Name implements markup.Parser
func (Parser) Name() string {
	return MarkupName
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return render(rawBytes, urlPrefix, metas, isWiki)
}
