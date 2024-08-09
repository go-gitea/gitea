// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bytes"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/regexplru"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates/vars"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"mvdan.cc/xurls/v2"
)

// Issue name styles
const (
	IssueNameStyleNumeric      = "numeric"
	IssueNameStyleAlphanumeric = "alphanumeric"
	IssueNameStyleRegexp       = "regexp"
)

var (
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// valid chars in encoded path and parameter: [-+~_%.a-zA-Z0-9/]

	// hashCurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, SHA256 are 64, the regex matches the hash from 7 to 64 chars in length
	// so that abbreviated hash links can be used as well. This matches git and GitHub usability.
	hashCurrentPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-f]{7,64})(?:\s|$|\)|\]|[.,:](\s|$))`)

	// shortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	shortLinkPattern = regexp.MustCompile(`\[\[(.*?)\]\](\w*)`)

	// anyHashPattern splits url containing SHA into parts
	anyHashPattern = regexp.MustCompile(`https?://(?:\S+/){4,5}([0-9a-f]{40,64})(/[-+~%./\w]+)?(\?[-+~%.\w&=]+)?(#[-+~%.\w]+)?`)

	// comparePattern matches "http://domain/org/repo/compare/COMMIT1...COMMIT2#hash"
	comparePattern = regexp.MustCompile(`https?://(?:\S+/){4,5}([0-9a-f]{7,64})(\.\.\.?)([0-9a-f]{7,64})?(#[-+~_%.a-zA-Z0-9]+)?`)

	// fullURLPattern matches full URL like "mailto:...", "https://..." and "ssh+git://..."
	fullURLPattern = regexp.MustCompile(`^[a-z][-+\w]+:`)

	// emailRegex is definitely not perfect with edge cases,
	// it is still accepted by the CommonMark specification, as well as the HTML5 spec:
	//   http://spec.commonmark.org/0.28/#email-address
	//   https://html.spec.whatwg.org/multipage/input.html#e-mail-state-(type%3Demail)
	emailRegex = regexp.MustCompile("(?:\\s|^|\\(|\\[)([a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9]{2,}(?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+)(?:\\s|$|\\)|\\]|;|,|\\?|!|\\.(\\s|$))")

	// blackfridayExtRegex is for blackfriday extensions create IDs like fn:user-content-footnote
	blackfridayExtRegex = regexp.MustCompile(`[^:]*:user-content-`)

	// emojiShortCodeRegex find emoji by alias like :smile:
	emojiShortCodeRegex = regexp.MustCompile(`:[-+\w]+:`)
)

// CSS class for action keywords (e.g. "closes: #1")
const keywordClass = "issue-keyword"

// IsFullURLBytes reports whether link fits valid format.
func IsFullURLBytes(link []byte) bool {
	return fullURLPattern.Match(link)
}

func IsFullURLString(link string) bool {
	return fullURLPattern.MatchString(link)
}

func IsNonEmptyRelativePath(link string) bool {
	return link != "" && !IsFullURLString(link) && link[0] != '/' && link[0] != '?' && link[0] != '#'
}

// regexp for full links to issues/pulls
var issueFullPattern *regexp.Regexp

// Once for to prevent races
var issueFullPatternOnce sync.Once

// regexp for full links to hash comment in pull request files changed tab
var filesChangedFullPattern *regexp.Regexp

// Once for to prevent races
var filesChangedFullPatternOnce sync.Once

func getIssueFullPattern() *regexp.Regexp {
	issueFullPatternOnce.Do(func() {
		// example: https://domain/org/repo/pulls/27#hash
		issueFullPattern = regexp.MustCompile(regexp.QuoteMeta(setting.AppURL) +
			`[\w_.-]+/[\w_.-]+/(?:issues|pulls)/((?:\w{1,10}-)?[1-9][0-9]*)([\?|#](\S+)?)?\b`)
	})
	return issueFullPattern
}

func getFilesChangedFullPattern() *regexp.Regexp {
	filesChangedFullPatternOnce.Do(func() {
		// example: https://domain/org/repo/pulls/27/files#hash
		filesChangedFullPattern = regexp.MustCompile(regexp.QuoteMeta(setting.AppURL) +
			`[\w_.-]+/[\w_.-]+/pulls/((?:\w{1,10}-)?[1-9][0-9]*)/files([\?|#](\S+)?)?\b`)
	})
	return filesChangedFullPattern
}

// CustomLinkURLSchemes allows for additional schemes to be detected when parsing links within text
func CustomLinkURLSchemes(schemes []string) {
	schemes = append(schemes, "http", "https")
	withAuth := make([]string, 0, len(schemes))
	validScheme := regexp.MustCompile(`^[a-z]+$`)
	for _, s := range schemes {
		if !validScheme.MatchString(s) {
			continue
		}
		without := false
		for _, sna := range xurls.SchemesNoAuthority {
			if s == sna {
				without = true
				break
			}
		}
		if without {
			s += ":"
		} else {
			s += "://"
		}
		withAuth = append(withAuth, s)
	}
	common.LinkRegex, _ = xurls.StrictMatchingScheme(strings.Join(withAuth, "|"))
}

type postProcessError struct {
	context string
	err     error
}

func (p *postProcessError) Error() string {
	return "PostProcess: " + p.context + ", " + p.err.Error()
}

type processor func(ctx *RenderContext, node *html.Node)

var defaultProcessors = []processor{
	fullIssuePatternProcessor,
	comparePatternProcessor,
	codePreviewPatternProcessor,
	fullHashPatternProcessor,
	shortLinkProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	emailAddressProcessor,
	emojiProcessor,
	emojiShortCodeProcessor,
}

// PostProcess does the final required transformations to the passed raw HTML
// data, and ensures its validity. Transformations include: replacing links and
// emails with HTML links, parsing shortlinks in the format of [[Link]], like
// MediaWiki, linking issues in the format #ID, and mentions in the format
// @user, and others.
func PostProcess(
	ctx *RenderContext,
	input io.Reader,
	output io.Writer,
) error {
	return postProcess(ctx, defaultProcessors, input, output)
}

var commitMessageProcessors = []processor{
	fullIssuePatternProcessor,
	comparePatternProcessor,
	fullHashPatternProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	emailAddressProcessor,
	emojiProcessor,
	emojiShortCodeProcessor,
}

// RenderCommitMessage will use the same logic as PostProcess, but will disable
// the shortLinkProcessor and will add a defaultLinkProcessor if defaultLink is
// set, which changes every text node into a link to the passed default link.
func RenderCommitMessage(
	ctx *RenderContext,
	content string,
) (string, error) {
	procs := commitMessageProcessors
	if ctx.DefaultLink != "" {
		// we don't have to fear data races, because being
		// commitMessageProcessors of fixed len and cap, every time we append
		// something to it the slice is realloc+copied, so append always
		// generates the slice ex-novo.
		procs = append(procs, genDefaultLinkProcessor(ctx.DefaultLink))
	}
	return renderProcessString(ctx, procs, content)
}

var commitMessageSubjectProcessors = []processor{
	fullIssuePatternProcessor,
	comparePatternProcessor,
	fullHashPatternProcessor,
	linkProcessor,
	mentionProcessor,
	issueIndexPatternProcessor,
	commitCrossReferencePatternProcessor,
	hashCurrentPatternProcessor,
	emojiShortCodeProcessor,
	emojiProcessor,
}

var emojiProcessors = []processor{
	emojiShortCodeProcessor,
	emojiProcessor,
}

// RenderCommitMessageSubject will use the same logic as PostProcess and
// RenderCommitMessage, but will disable the shortLinkProcessor and
// emailAddressProcessor, will add a defaultLinkProcessor if defaultLink is set,
// which changes every text node into a link to the passed default link.
func RenderCommitMessageSubject(
	ctx *RenderContext,
	content string,
) (string, error) {
	procs := commitMessageSubjectProcessors
	if ctx.DefaultLink != "" {
		// we don't have to fear data races, because being
		// commitMessageSubjectProcessors of fixed len and cap, every time we
		// append something to it the slice is realloc+copied, so append always
		// generates the slice ex-novo.
		procs = append(procs, genDefaultLinkProcessor(ctx.DefaultLink))
	}
	return renderProcessString(ctx, procs, content)
}

// RenderIssueTitle to process title on individual issue/pull page
func RenderIssueTitle(
	ctx *RenderContext,
	title string,
) (string, error) {
	return renderProcessString(ctx, []processor{
		issueIndexPatternProcessor,
		commitCrossReferencePatternProcessor,
		hashCurrentPatternProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, title)
}

func renderProcessString(ctx *RenderContext, procs []processor, content string) (string, error) {
	var buf strings.Builder
	if err := postProcess(ctx, procs, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderDescriptionHTML will use similar logic as PostProcess, but will
// use a single special linkProcessor.
func RenderDescriptionHTML(
	ctx *RenderContext,
	content string,
) (string, error) {
	return renderProcessString(ctx, []processor{
		descriptionLinkProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, content)
}

// RenderEmoji for when we want to just process emoji and shortcodes
// in various places it isn't already run through the normal markdown processor
func RenderEmoji(
	ctx *RenderContext,
	content string,
) (string, error) {
	return renderProcessString(ctx, emojiProcessors, content)
}

var (
	tagCleaner = regexp.MustCompile(`<((?:/?\w+/\w+)|(?:/[\w ]+/)|(/?[hH][tT][mM][lL]\b)|(/?[hH][eE][aA][dD]\b))`)
	nulCleaner = strings.NewReplacer("\000", "")
)

func postProcess(ctx *RenderContext, procs []processor, input io.Reader, output io.Writer) error {
	defer ctx.Cancel()
	// FIXME: don't read all content to memory
	rawHTML, err := io.ReadAll(input)
	if err != nil {
		return err
	}

	// parse the HTML
	node, err := html.Parse(io.MultiReader(
		// prepend "<html><body>"
		strings.NewReader("<html><body>"),
		// Strip out nuls - they're always invalid
		bytes.NewReader(tagCleaner.ReplaceAll([]byte(nulCleaner.Replace(string(rawHTML))), []byte("&lt;$1"))),
		// close the tags
		strings.NewReader("</body></html>"),
	))
	if err != nil {
		return &postProcessError{"invalid HTML", err}
	}

	if node.Type == html.DocumentNode {
		node = node.FirstChild
	}

	visitNode(ctx, procs, node)

	newNodes := make([]*html.Node, 0, 5)

	if node.Data == "html" {
		node = node.FirstChild
		for node != nil && node.Data != "body" {
			node = node.NextSibling
		}
	}
	if node != nil {
		if node.Data == "body" {
			child := node.FirstChild
			for child != nil {
				newNodes = append(newNodes, child)
				child = child.NextSibling
			}
		} else {
			newNodes = append(newNodes, node)
		}
	}

	// Render everything to buf.
	for _, node := range newNodes {
		if err := html.Render(output, node); err != nil {
			return &postProcessError{"error rendering processed HTML", err}
		}
	}
	return nil
}

func visitNode(ctx *RenderContext, procs []processor, node *html.Node) *html.Node {
	// Add user-content- to IDs and "#" links if they don't already have them
	for idx, attr := range node.Attr {
		val := strings.TrimPrefix(attr.Val, "#")
		notHasPrefix := !(strings.HasPrefix(val, "user-content-") || blackfridayExtRegex.MatchString(val))

		if attr.Key == "id" && notHasPrefix {
			node.Attr[idx].Val = "user-content-" + attr.Val
		}

		if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") && notHasPrefix {
			node.Attr[idx].Val = "#user-content-" + val
		}

		if attr.Key == "class" && attr.Val == "emoji" {
			procs = nil
		}
	}

	switch node.Type {
	case html.TextNode:
		processTextNodes(ctx, procs, node)
	case html.ElementNode:
		if node.Data == "code" || node.Data == "pre" {
			// ignore code and pre nodes
			return node.NextSibling
		} else if node.Data == "img" {
			return visitNodeImg(ctx, node)
		} else if node.Data == "video" {
			return visitNodeVideo(ctx, node)
		} else if node.Data == "a" {
			// Restrict text in links to emojis
			procs = emojiProcessors
		} else if node.Data == "i" {
			for _, attr := range node.Attr {
				if attr.Key != "class" {
					continue
				}
				classes := strings.Split(attr.Val, " ")
				for i, class := range classes {
					if class == "icon" {
						classes[0], classes[i] = classes[i], classes[0]
						attr.Val = strings.Join(classes, " ")

						// Remove all children of icons
						child := node.FirstChild
						for child != nil {
							node.RemoveChild(child)
							child = node.FirstChild
						}
						break
					}
				}
			}
		}
		for n := node.FirstChild; n != nil; {
			n = visitNode(ctx, procs, n)
		}
	default:
	}
	return node.NextSibling
}

// processTextNodes runs the passed node through various processors, in order to handle
// all kinds of special links handled by the post-processing.
func processTextNodes(ctx *RenderContext, procs []processor, node *html.Node) {
	for _, p := range procs {
		p(ctx, node)
	}
}

// createKeyword() renders a highlighted version of an action keyword
func createKeyword(content string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: keywordClass})

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	span.AppendChild(text)

	return span
}

func createEmoji(content, class, name string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	if class != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: class})
	}
	if name != "" {
		span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: name})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	span.AppendChild(text)
	return span
}

func createCustomEmoji(alias string) *html.Node {
	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, html.Attribute{Key: "class", Val: "emoji"})
	span.Attr = append(span.Attr, html.Attribute{Key: "aria-label", Val: alias})

	img := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Img,
		Data:     "img",
		Attr:     []html.Attribute{},
	}
	img.Attr = append(img.Attr, html.Attribute{Key: "alt", Val: ":" + alias + ":"})
	img.Attr = append(img.Attr, html.Attribute{Key: "src", Val: setting.StaticURLPrefix + "/assets/img/emoji/" + alias + ".png"})

	span.AppendChild(img)
	return span
}

func createLink(href, content, class string) *html.Node {
	a := &html.Node{
		Type: html.ElementNode,
		Data: atom.A.String(),
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}

	if class != "" {
		a.Attr = append(a.Attr, html.Attribute{Key: "class", Val: class})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	a.AppendChild(text)
	return a
}

func createCodeLink(href, content, class string) *html.Node {
	a := &html.Node{
		Type: html.ElementNode,
		Data: atom.A.String(),
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}

	if class != "" {
		a.Attr = append(a.Attr, html.Attribute{Key: "class", Val: class})
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	code := &html.Node{
		Type: html.ElementNode,
		Data: atom.Code.String(),
		Attr: []html.Attribute{{Key: "class", Val: "nohighlight"}},
	}

	code.AppendChild(text)
	a.AppendChild(code)
	return a
}

// replaceContent takes text node, and in its content it replaces a section of
// it with the specified newNode.
func replaceContent(node *html.Node, i, j int, newNode *html.Node) {
	replaceContentList(node, i, j, []*html.Node{newNode})
}

// replaceContentList takes text node, and in its content it replaces a section of
// it with the specified newNodes. An example to visualize how this can work can
// be found here: https://play.golang.org/p/5zP8NnHZ03s
func replaceContentList(node *html.Node, i, j int, newNodes []*html.Node) {
	// get the data before and after the match
	before := node.Data[:i]
	after := node.Data[j:]

	// Replace in the current node the text, so that it is only what it is
	// supposed to have.
	node.Data = before

	// Get the current next sibling, before which we place the replaced data,
	// and after that we place the new text node.
	nextSibling := node.NextSibling
	for _, n := range newNodes {
		node.Parent.InsertBefore(n, nextSibling)
	}
	if after != "" {
		node.Parent.InsertBefore(&html.Node{
			Type: html.TextNode,
			Data: after,
		}, nextSibling)
	}
}

func mentionProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	nodeStop := node.NextSibling
	for node != nodeStop {
		found, loc := references.FindFirstMentionBytes(util.UnsafeStringToBytes(node.Data[start:]))
		if !found {
			node = node.NextSibling
			start = 0
			continue
		}
		loc.Start += start
		loc.End += start
		mention := node.Data[loc.Start:loc.End]
		teams, ok := ctx.Metas["teams"]
		// FIXME: util.URLJoin may not be necessary here:
		// - setting.AppURL is defined to have a terminal '/' so unless mention[1:]
		// is an AppSubURL link we can probably fallback to concatenation.
		// team mention should follow @orgName/teamName style
		if ok && strings.Contains(mention, "/") {
			mentionOrgAndTeam := strings.Split(mention, "/")
			if mentionOrgAndTeam[0][1:] == ctx.Metas["org"] && strings.Contains(teams, ","+strings.ToLower(mentionOrgAndTeam[1])+",") {
				replaceContent(node, loc.Start, loc.End, createLink(util.URLJoin(ctx.Links.Prefix(), "org", ctx.Metas["org"], "teams", mentionOrgAndTeam[1]), mention, "mention"))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			start = loc.End
			continue
		}
		mentionedUsername := mention[1:]

		if DefaultProcessorHelper.IsUsernameMentionable != nil && DefaultProcessorHelper.IsUsernameMentionable(ctx.Ctx, mentionedUsername) {
			replaceContent(node, loc.Start, loc.End, createLink(util.URLJoin(ctx.Links.Prefix(), mentionedUsername), mention, "mention"))
			node = node.NextSibling.NextSibling
			start = 0
		} else {
			start = loc.End
		}
	}
}

func shortLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := shortLinkPattern.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		content := node.Data[m[2]:m[3]]
		tail := node.Data[m[4]:m[5]]
		props := make(map[string]string)

		// MediaWiki uses [[link|text]], while GitHub uses [[text|link]]
		// It makes page handling terrible, but we prefer GitHub syntax
		// And fall back to MediaWiki only when it is obvious from the look
		// Of text and link contents
		sl := strings.Split(content, "|")
		for _, v := range sl {
			if equalPos := strings.IndexByte(v, '='); equalPos == -1 {
				// There is no equal in this argument; this is a mandatory arg
				if props["name"] == "" {
					if IsFullURLString(v) {
						// If we clearly see it is a link, we save it so

						// But first we need to ensure, that if both mandatory args provided
						// look like links, we stick to GitHub syntax
						if props["link"] != "" {
							props["name"] = props["link"]
						}

						props["link"] = strings.TrimSpace(v)
					} else {
						props["name"] = v
					}
				} else {
					props["link"] = strings.TrimSpace(v)
				}
			} else {
				// There is an equal; optional argument.

				sep := strings.IndexByte(v, '=')
				key, val := v[:sep], html.UnescapeString(v[sep+1:])

				// When parsing HTML, x/net/html will change all quotes which are
				// not used for syntax into UTF-8 quotes. So checking val[0] won't
				// be enough, since that only checks a single byte.
				if len(val) > 1 {
					if (strings.HasPrefix(val, "“") && strings.HasSuffix(val, "”")) ||
						(strings.HasPrefix(val, "‘") && strings.HasSuffix(val, "’")) {
						const lenQuote = len("‘")
						val = val[lenQuote : len(val)-lenQuote]
					} else if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
						(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
						val = val[1 : len(val)-1]
					} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "’") {
						const lenQuote = len("‘")
						val = val[1 : len(val)-lenQuote]
					}
				}
				props[key] = val
			}
		}

		var name, link string
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

		name += tail
		image := false
		ext := filepath.Ext(link)
		switch ext {
		// fast path: empty string, ignore
		case "":
			// leave image as false
		case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".gif", ".bmp", ".ico", ".svg":
			image = true
		}

		childNode := &html.Node{}
		linkNode := &html.Node{
			FirstChild: childNode,
			LastChild:  childNode,
			Type:       html.ElementNode,
			Data:       "a",
			DataAtom:   atom.A,
		}
		childNode.Parent = linkNode
		absoluteLink := IsFullURLString(link)
		if !absoluteLink {
			if image {
				link = strings.ReplaceAll(link, " ", "+")
			} else {
				link = strings.ReplaceAll(link, " ", "-") // FIXME: it should support dashes in the link, eg: "the-dash-support.-"
			}
			if !strings.Contains(link, "/") {
				link = url.PathEscape(link) // FIXME: it doesn't seem right and it might cause double-escaping
			}
		}
		if image {
			if !absoluteLink {
				link = util.URLJoin(ctx.Links.ResolveMediaLink(ctx.IsWiki), link)
			}
			title := props["title"]
			if title == "" {
				title = props["alt"]
			}
			if title == "" {
				title = path.Base(name)
			}
			alt := props["alt"]
			if alt == "" {
				alt = name
			}

			// make the childNode an image - if we can, we also place the alt
			childNode.Type = html.ElementNode
			childNode.Data = "img"
			childNode.DataAtom = atom.Img
			childNode.Attr = []html.Attribute{
				{Key: "src", Val: link},
				{Key: "title", Val: title},
				{Key: "alt", Val: alt},
			}
			if alt == "" {
				childNode.Attr = childNode.Attr[:2]
			}
		} else {
			link, _ = ResolveLink(ctx, link, "")
			childNode.Type = html.TextNode
			childNode.Data = name
		}
		linkNode.Attr = []html.Attribute{{Key: "href", Val: link}}
		replaceContent(node, m[0], m[1], linkNode)
		node = node.NextSibling.NextSibling
	}
}

func fullIssuePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}
	next := node.NextSibling
	for node != nil && node != next {
		m := getIssueFullPattern().FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		mDiffView := getFilesChangedFullPattern().FindStringSubmatchIndex(node.Data)
		// leave it as it is if the link is from "Files Changed" tab in PR Diff View https://domain/org/repo/pulls/27/files
		if mDiffView != nil {
			return
		}

		link := node.Data[m[0]:m[1]]
		text := "#" + node.Data[m[2]:m[3]]
		// if m[4] and m[5] is not -1, then link is to a comment
		// indicate that in the text by appending (comment)
		if m[4] != -1 && m[5] != -1 {
			if locale, ok := ctx.Ctx.Value(translation.ContextKey).(translation.Locale); ok {
				text += " " + locale.TrString("repo.from_comment")
			} else {
				text += " (comment)"
			}
		}

		// extract repo and org name from matched link like
		// http://localhost:3000/gituser/myrepo/issues/1
		linkParts := strings.Split(link, "/")
		matchOrg := linkParts[len(linkParts)-4]
		matchRepo := linkParts[len(linkParts)-3]

		if matchOrg == ctx.Metas["user"] && matchRepo == ctx.Metas["repo"] {
			replaceContent(node, m[0], m[1], createLink(link, text, "ref-issue"))
		} else {
			text = matchOrg + "/" + matchRepo + text
			replaceContent(node, m[0], m[1], createLink(link, text, "ref-issue"))
		}
		node = node.NextSibling.NextSibling
	}
}

func issueIndexPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}

	// FIXME: the use of "mode" is quite dirty and hacky, for example: what is a "document"? how should it be rendered?
	// The "mode" approach should be refactored to some other more clear&reliable way.
	crossLinkOnly := ctx.Metas["mode"] == "document" && !ctx.IsWiki

	var (
		found bool
		ref   *references.RenderizableReference
	)

	next := node.NextSibling

	for node != nil && node != next {
		_, hasExtTrackFormat := ctx.Metas["format"]

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		isNumericStyle := ctx.Metas["style"] == "" || ctx.Metas["style"] == IssueNameStyleNumeric
		foundNumeric, refNumeric := references.FindRenderizableReferenceNumeric(node.Data, hasExtTrackFormat && !isNumericStyle, crossLinkOnly)

		switch ctx.Metas["style"] {
		case "", IssueNameStyleNumeric:
			found, ref = foundNumeric, refNumeric
		case IssueNameStyleAlphanumeric:
			found, ref = references.FindRenderizableReferenceAlphanumeric(node.Data)
		case IssueNameStyleRegexp:
			pattern, err := regexplru.GetCompiled(ctx.Metas["regexp"])
			if err != nil {
				return
			}
			found, ref = references.FindRenderizableReferenceRegexp(node.Data, pattern)
		}

		// Repos with external issue trackers might still need to reference local PRs
		// We need to concern with the first one that shows up in the text, whichever it is
		if hasExtTrackFormat && !isNumericStyle && refNumeric != nil {
			// If numeric (PR) was found, and it was BEFORE the non-numeric pattern, use that
			// Allow a free-pass when non-numeric pattern wasn't found.
			if found && (ref == nil || refNumeric.RefLocation.Start < ref.RefLocation.Start) {
				found = foundNumeric
				ref = refNumeric
			}
		}
		if !found {
			return
		}

		var link *html.Node
		reftext := node.Data[ref.RefLocation.Start:ref.RefLocation.End]
		if hasExtTrackFormat && !ref.IsPull {
			ctx.Metas["index"] = ref.Issue

			res, err := vars.Expand(ctx.Metas["format"], ctx.Metas)
			if err != nil {
				// here we could just log the error and continue the rendering
				log.Error("unable to expand template vars for ref %s, err: %v", ref.Issue, err)
			}

			link = createLink(res, reftext, "ref-issue ref-external-issue")
		} else {
			// Path determines the type of link that will be rendered. It's unknown at this point whether
			// the linked item is actually a PR or an issue. Luckily it's of no real consequence because
			// Gitea will redirect on click as appropriate.
			issuePath := util.Iif(ref.IsPull, "pulls", "issues")
			if ref.Owner == "" {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ctx.Metas["user"], ctx.Metas["repo"], issuePath, ref.Issue), reftext, "ref-issue")
			} else {
				link = createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, issuePath, ref.Issue), reftext, "ref-issue")
			}
		}

		if ref.Action == references.XRefActionNone {
			replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
			node = node.NextSibling.NextSibling
			continue
		}

		// Decorate action keywords if actionable
		var keyword *html.Node
		if references.IsXrefActionable(ref, hasExtTrackFormat) {
			keyword = createKeyword(node.Data[ref.ActionLocation.Start:ref.ActionLocation.End])
		} else {
			keyword = &html.Node{
				Type: html.TextNode,
				Data: node.Data[ref.ActionLocation.Start:ref.ActionLocation.End],
			}
		}
		spaces := &html.Node{
			Type: html.TextNode,
			Data: node.Data[ref.ActionLocation.End:ref.RefLocation.Start],
		}
		replaceContentList(node, ref.ActionLocation.Start, ref.RefLocation.End, []*html.Node{keyword, spaces, link})
		node = node.NextSibling.NextSibling.NextSibling.NextSibling
	}
}

func commitCrossReferencePatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling

	for node != nil && node != next {
		found, ref := references.FindRenderizableCommitCrossReference(node.Data)
		if !found {
			return
		}

		reftext := ref.Owner + "/" + ref.Name + "@" + base.ShortSha(ref.CommitSha)
		link := createLink(util.URLJoin(ctx.Links.Prefix(), ref.Owner, ref.Name, "commit", ref.CommitSha), reftext, "commit")

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}

type anyHashPatternResult struct {
	PosStart  int
	PosEnd    int
	FullURL   string
	CommitID  string
	SubPath   string
	QueryHash string
}

func anyHashPatternExtract(s string) (ret anyHashPatternResult, ok bool) {
	m := anyHashPattern.FindStringSubmatchIndex(s)
	if m == nil {
		return ret, false
	}

	ret.PosStart, ret.PosEnd = m[0], m[1]
	ret.FullURL = s[ret.PosStart:ret.PosEnd]
	if strings.HasSuffix(ret.FullURL, ".") {
		// if url ends in '.', it's very likely that it is not part of the actual url but used to finish a sentence.
		ret.PosEnd--
		ret.FullURL = ret.FullURL[:len(ret.FullURL)-1]
		for i := 0; i < len(m); i++ {
			m[i] = min(m[i], ret.PosEnd)
		}
	}

	ret.CommitID = s[m[2]:m[3]]
	if m[5] > 0 {
		ret.SubPath = s[m[4]:m[5]]
	}

	lastStart, lastEnd := m[len(m)-2], m[len(m)-1]
	if lastEnd > 0 {
		ret.QueryHash = s[lastStart:lastEnd][1:]
	}
	return ret, true
}

// fullHashPatternProcessor renders SHA containing URLs
func fullHashPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		ret, ok := anyHashPatternExtract(node.Data)
		if !ok {
			node = node.NextSibling
			continue
		}
		text := base.ShortSha(ret.CommitID)
		if ret.SubPath != "" {
			text += ret.SubPath
		}
		if ret.QueryHash != "" {
			text += " (" + ret.QueryHash + ")"
		}
		replaceContent(node, ret.PosStart, ret.PosEnd, createCodeLink(ret.FullURL, text, "commit"))
		node = node.NextSibling.NextSibling
	}
}

func comparePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		m := comparePattern.FindStringSubmatchIndex(node.Data)
		if m == nil || slices.Contains(m[:8], -1) { // ensure that every group (m[0]...m[7]) has a match
			node = node.NextSibling
			continue
		}

		urlFull := node.Data[m[0]:m[1]]
		text1 := base.ShortSha(node.Data[m[2]:m[3]])
		textDots := base.ShortSha(node.Data[m[4]:m[5]])
		text2 := base.ShortSha(node.Data[m[6]:m[7]])

		hash := ""
		if m[9] > 0 {
			hash = node.Data[m[8]:m[9]][1:]
		}

		start := m[0]
		end := m[1]

		// If url ends in '.', it's very likely that it is not part of the
		// actual url but used to finish a sentence.
		if strings.HasSuffix(urlFull, ".") {
			end--
			urlFull = urlFull[:len(urlFull)-1]
			if hash != "" {
				hash = hash[:len(hash)-1]
			} else if text2 != "" {
				text2 = text2[:len(text2)-1]
			}
		}

		text := text1 + textDots + text2
		if hash != "" {
			text += " (" + hash + ")"
		}
		replaceContent(node, start, end, createCodeLink(urlFull, text, "compare"))
		node = node.NextSibling.NextSibling
	}
}

// emojiShortCodeProcessor for rendering text like :smile: into emoji
func emojiShortCodeProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := emojiShortCodeRegex.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		start = m[1]

		alias := node.Data[m[0]:m[1]]
		alias = strings.ReplaceAll(alias, ":", "")
		converted := emoji.FromAlias(alias)
		if converted == nil {
			// check if this is a custom reaction
			if _, exist := setting.UI.CustomEmojisMap[alias]; exist {
				replaceContent(node, m[0], m[1], createCustomEmoji(alias))
				node = node.NextSibling.NextSibling
				start = 0
				continue
			}
			continue
		}

		replaceContent(node, m[0], m[1], createEmoji(converted.Emoji, "emoji", converted.Description))
		node = node.NextSibling.NextSibling
		start = 0
	}
}

// emoji processor to match emoji and add emoji class
func emojiProcessor(ctx *RenderContext, node *html.Node) {
	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := emoji.FindEmojiSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[0] += start
		m[1] += start

		codepoint := node.Data[m[0]:m[1]]
		start = m[1]
		val := emoji.FromCode(codepoint)
		if val != nil {
			replaceContent(node, m[0], m[1], createEmoji(codepoint, "emoji", val.Description))
			node = node.NextSibling.NextSibling
			start = 0
		}
	}
}

// hashCurrentPatternProcessor renders SHA1 strings to corresponding links that
// are assumed to be in the same repository.
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.Metas == nil || ctx.Metas["user"] == "" || ctx.Metas["repo"] == "" || (ctx.Repo == nil && ctx.GitRepo == nil) {
		return
	}

	start := 0
	next := node.NextSibling
	if ctx.ShaExistCache == nil {
		ctx.ShaExistCache = make(map[string]bool)
	}
	for node != nil && node != next && start < len(node.Data) {
		m := hashCurrentPattern.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		m[2] += start
		m[3] += start

		hash := node.Data[m[2]:m[3]]
		// The regex does not lie, it matches the hash pattern.
		// However, a regex cannot know if a hash actually exists or not.
		// We could assume that a SHA1 hash should probably contain alphas AND numerics
		// but that is not always the case.
		// Although unlikely, deadbeef and 1234567 are valid short forms of SHA1 hash
		// as used by git and github for linking and thus we have to do similar.
		// Because of this, we check to make sure that a matched hash is actually
		// a commit in the repository before making it a link.

		// check cache first
		exist, inCache := ctx.ShaExistCache[hash]
		if !inCache {
			if ctx.GitRepo == nil {
				var err error
				var closer io.Closer
				ctx.GitRepo, closer, err = gitrepo.RepositoryFromContextOrOpen(ctx.Ctx, ctx.Repo)
				if err != nil {
					log.Error("unable to open repository: %s Error: %v", gitrepo.RepoGitURL(ctx.Repo), err)
					return
				}
				ctx.AddCancel(func() {
					_ = closer.Close()
					ctx.GitRepo = nil
				})
			}

			exist = ctx.GitRepo.IsObjectExist(hash)
			ctx.ShaExistCache[hash] = exist
		}

		if !exist {
			start = m[3]
			continue
		}

		link := util.URLJoin(ctx.Links.Prefix(), ctx.Metas["user"], ctx.Metas["repo"], "commit", hash)
		replaceContent(node, m[2], m[3], createCodeLink(link, base.ShortSha(hash), "commit"))
		start = 0
		node = node.NextSibling.NextSibling
	}
}

// emailAddressProcessor replaces raw email addresses with a mailto: link.
func emailAddressProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := emailRegex.FindStringSubmatchIndex(node.Data)
		if m == nil {
			return
		}

		mail := node.Data[m[2]:m[3]]
		replaceContent(node, m[2], m[3], createLink("mailto:"+mail, mail, "mailto"))
		node = node.NextSibling.NextSibling
	}
}

// linkProcessor creates links for any HTTP or HTTPS URL not captured by
// markdown.
func linkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		replaceContent(node, m[0], m[1], createLink(uri, uri, "link"))
		node = node.NextSibling.NextSibling
	}
}

func genDefaultLinkProcessor(defaultLink string) processor {
	return func(ctx *RenderContext, node *html.Node) {
		ch := &html.Node{
			Parent: node,
			Type:   html.TextNode,
			Data:   node.Data,
		}

		node.Type = html.ElementNode
		node.Data = "a"
		node.DataAtom = atom.A
		node.Attr = []html.Attribute{
			{Key: "href", Val: defaultLink},
			{Key: "class", Val: "default-link muted"},
		}
		node.FirstChild, node.LastChild = ch, ch
	}
}

// descriptionLinkProcessor creates links for DescriptionHTML
func descriptionLinkProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling
	for node != nil && node != next {
		m := common.LinkRegex.FindStringIndex(node.Data)
		if m == nil {
			return
		}

		uri := node.Data[m[0]:m[1]]
		replaceContent(node, m[0], m[1], createDescriptionLink(uri, uri))
		node = node.NextSibling.NextSibling
	}
}

func createDescriptionLink(href, content string) *html.Node {
	textNode := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	linkNode := &html.Node{
		FirstChild: textNode,
		LastChild:  textNode,
		Type:       html.ElementNode,
		Data:       "a",
		DataAtom:   atom.A,
		Attr: []html.Attribute{
			{Key: "href", Val: href},
			{Key: "target", Val: "_blank"},
			{Key: "rel", Val: "noopener noreferrer"},
		},
	}
	textNode.Parent = linkNode
	return linkNode
}
