// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

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
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
	"golang.org/x/net/html"
)

// Issue name styles
const (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
)

var (
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// MentionPattern matches string that mentions someone, e.g. @Unknwon
	MentionPattern = regexp.MustCompile(`(\s|^|\W)@[0-9a-zA-Z-_\.]+`)

	// IssueNumericPattern matches string that references to a numeric issue, e.g. #1287
	IssueNumericPattern = regexp.MustCompile(`( |^|\(|\[)#[0-9]+\b`)
	// IssueAlphanumericPattern matches string that references to an alphanumeric issue, e.g. ABC-1234
	IssueAlphanumericPattern = regexp.MustCompile(`( |^|\(|\[)[A-Z]{1,10}-[1-9][0-9]*\b`)
	// CrossReferenceIssueNumericPattern matches string that references a numeric issue in a different repository
	// e.g. gogits/gogs#12345
	CrossReferenceIssueNumericPattern = regexp.MustCompile(`( |^)[0-9a-zA-Z-_\.]+/[0-9a-zA-Z-_\.]+#[0-9]+\b`)

	// Sha1CurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, the regex matches the hash from 7 to 40 chars in length
	// so that abbreviated hash links can be used as well. This matches git and github useability.
	Sha1CurrentPattern = regexp.MustCompile(`(?:^|\s|\()([0-9a-f]{7,40})\b`)

	// ShortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	ShortLinkPattern = regexp.MustCompile(`(\[\[.*?\]\]\w*)`)

	// AnySHA1Pattern allows to split url containing SHA into parts
	AnySHA1Pattern = regexp.MustCompile(`(http\S*)://(\S+)/(\S+)/(\S+)/(\S+)/([0-9a-f]{40})(?:/?([^#\s]+)?(?:#(\S+))?)?`)

	validLinksPattern = regexp.MustCompile(`^[a-z][\w-]+://`)
)

// regexp for full links to issues/pulls
var issueFullPattern *regexp.Regexp

// IsLink reports whether link fits valid format.
func IsLink(link []byte) bool {
	return isLink(link)
}

// isLink reports whether link fits valid format.
func isLink(link []byte) bool {
	return validLinksPattern.Match(link)
}

func getIssueFullPattern() *regexp.Regexp {
	if issueFullPattern == nil {
		appURL := setting.AppURL
		if len(appURL) > 0 && appURL[len(appURL)-1] != '/' {
			appURL += "/"
		}
		issueFullPattern = regexp.MustCompile(appURL +
			`\w+/\w+/(?:issues|pulls)/((?:\w{1,10}-)?[1-9][0-9]*)([\?|#]\S+.(\S+)?)?\b`)
	}
	return issueFullPattern
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

// RenderIssueIndexPatternOptions options for RenderIssueIndexPattern function
type RenderIssueIndexPatternOptions struct {
	// url to which non-special formatting should be linked. If empty,
	// no such links will be added
	DefaultURL string
	URLPrefix  string
	Metas      map[string]string
}

// addText add text to the given buffer, adding a link to the default url
// if appropriate
func (opts RenderIssueIndexPatternOptions) addText(text []byte, buf *bytes.Buffer) {
	if len(text) == 0 {
		return
	} else if len(opts.DefaultURL) == 0 {
		buf.Write(text)
		return
	}
	buf.WriteString(`<a rel="nofollow" href="`)
	buf.WriteString(opts.DefaultURL)
	buf.WriteString(`">`)
	buf.Write(text)
	buf.WriteString(`</a>`)
}

// RenderIssueIndexPattern renders issue indexes to corresponding links.
func RenderIssueIndexPattern(rawBytes []byte, opts RenderIssueIndexPatternOptions) []byte {
	opts.URLPrefix = cutoutVerbosePrefix(opts.URLPrefix)

	pattern := IssueNumericPattern
	if opts.Metas["style"] == IssueNameStyleAlphanumeric {
		pattern = IssueAlphanumericPattern
	}

	var buf bytes.Buffer
	remainder := rawBytes
	for {
		indices := pattern.FindIndex(remainder)
		if indices == nil || len(indices) < 2 {
			opts.addText(remainder, &buf)
			return buf.Bytes()
		}
		startIndex := indices[0]
		endIndex := indices[1]
		opts.addText(remainder[:startIndex], &buf)
		if remainder[startIndex] == '(' || remainder[startIndex] == ' ' {
			buf.WriteByte(remainder[startIndex])
			startIndex++
		}
		if opts.Metas == nil {
			buf.WriteString(`<a href="`)
			buf.WriteString(URLJoin(
				opts.URLPrefix, "issues", string(remainder[startIndex+1:endIndex])))
			buf.WriteString(`">`)
			buf.Write(remainder[startIndex:endIndex])
			buf.WriteString(`</a>`)
		} else {
			// Support for external issue tracker
			buf.WriteString(`<a href="`)
			if opts.Metas["style"] == IssueNameStyleAlphanumeric {
				opts.Metas["index"] = string(remainder[startIndex:endIndex])
			} else {
				opts.Metas["index"] = string(remainder[startIndex+1 : endIndex])
			}
			buf.WriteString(com.Expand(opts.Metas["format"], opts.Metas))
			buf.WriteString(`">`)
			buf.Write(remainder[startIndex:endIndex])
			buf.WriteString(`</a>`)
		}
		if endIndex < len(remainder) &&
			(remainder[endIndex] == ')' || remainder[endIndex] == ' ') {
			buf.WriteByte(remainder[endIndex])
			endIndex++
		}
		remainder = remainder[endIndex:]
	}
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

// RenderFullIssuePattern renders issues-like URLs
func RenderFullIssuePattern(rawBytes []byte) []byte {
	ms := getIssueFullPattern().FindAllSubmatch(rawBytes, -1)
	for _, m := range ms {
		all := m[0]
		id := string(m[1])
		text := "#" + id
		// TODO if m[2] is not nil, then link is to a comment,
		// and we should indicate that in the text somehow
		rawBytes = bytes.Replace(rawBytes, all, []byte(fmt.Sprintf(
			`<a href="%s">%s</a>`, string(all), text)), -1)
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

	rawBytes = RenderFullIssuePattern(rawBytes)
	rawBytes = RenderShortLinks(rawBytes, urlPrefix, false, isWikiMarkdown)
	rawBytes = RenderIssueIndexPattern(rawBytes, RenderIssueIndexPatternOptions{
		URLPrefix: urlPrefix,
		Metas:     metas,
	})
	rawBytes = RenderCrossReferenceIssueIndexPattern(rawBytes, urlPrefix, metas)
	rawBytes = renderFullSha1Pattern(rawBytes, urlPrefix)
	rawBytes = renderSha1CurrentPattern(rawBytes, urlPrefix)
	return rawBytes
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
