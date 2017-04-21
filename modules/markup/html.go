// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/setting"

	"github.com/Unknwon/com"
)

var (
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
	// FIXME: this pattern matches pure numbers as well, right now we do a hack to check in renderSha1CurrentPattern
	// by converting string to a number.
	Sha1CurrentPattern = regexp.MustCompile(`(?:^|\s|\()[0-9a-f]{40}\b`)

	// ShortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	ShortLinkPattern = regexp.MustCompile(`(\[\[.*\]\]\w*)`)

	// AnySHA1Pattern allows to split url containing SHA into parts
	AnySHA1Pattern = regexp.MustCompile(`(http\S*)://(\S+)/(\S+)/(\S+)/(\S+)/([0-9a-f]{40})(?:/?([^#\s]+)?(?:#(\S+))?)?`)

	// IssueFullPattern allows to split issue (and pull) URLs into parts
	IssueFullPattern = regexp.MustCompile(`(?:^|\s|\()(http\S*)://((?:[^\s/]+/)+)((?:\w{1,10}-)?[1-9][0-9]*)([\?|#]\S+.(\S+)?)?\b`)

	validLinksPattern = regexp.MustCompile(`^[a-z][\w-]+://`)

	leftAngleBracket  = []byte("</")
	rightAngleBracket = []byte(">")

	noEndTags = []string{"img", "input", "br", "hr"}
)

// Issue name styles
var (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
)

// IsLink reports whether link fits valid format.
func IsLink(link []byte) bool {
	return validLinksPattern.Match(link)
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

// FindAllMentions matches mention patterns in given content
// and returns a list of found user names without @ prefix.
func FindAllMentions(content string) []string {
	mentions := MentionPattern.FindAllString(content, -1)
	for i := range mentions {
		mentions[i] = mentions[i][strings.Index(mentions[i], "@")+1:] // Strip @ character
	}
	return mentions
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
// isWiki is a flag to choose linking url prefix
// TODO: this is markdown special syntax??? so let's move this to markdown package
func RenderShortLinks(rawBytes []byte, urlPrefix string, noLink bool, isWiki bool) []byte {
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
						if IsLink(v) {
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
		absoluteLink := IsLink([]byte(link))
		if !absoluteLink {
			link = strings.Replace(link, " ", "+", -1)
		}
		if image {
			if !absoluteLink {
				if IsSameDomain(urlPrefix) {
					urlPrefix = strings.Replace(urlPrefix, "/src/", "/raw/", 1)
				}
				if isWiki {
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
			if isWiki {
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
		all := m[0]
		if com.StrTo(all).MustInt() > 0 {
			continue
		}
		rawBytes = bytes.Replace(rawBytes, all, []byte(fmt.Sprintf(
			`<a href="%s">%s</a>`, URLJoin(urlPrefix, "commit", string(all)), base.ShortSha(string(all)))), -1)
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

// URLJoin joins url components, like path.Join, but preserving contents
func URLJoin(elem ...string) string {
	res := ""
	last := len(elem) - 1
	for i, item := range elem {
		res += item
		if i != last && !strings.HasSuffix(res, "/") {
			res += "/"
		}
	}
	cwdIndex := strings.Index(res, "/./")
	for cwdIndex != -1 {
		res = strings.Replace(res, "/./", "/", 1)
		cwdIndex = strings.Index(res, "/./")
	}
	upIndex := strings.Index(res, "/..")
	for upIndex != -1 {
		res = strings.Replace(res, "/..", "", 1)
		prevStart := -1
		for i := upIndex - 1; i >= 0; i-- {
			if res[i] == '/' {
				prevStart = i
				break
			}
		}
		if prevStart != -1 {
			res = res[:prevStart] + res[upIndex:]
		}
		upIndex = strings.Index(res, "/..")
	}
	return res
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
