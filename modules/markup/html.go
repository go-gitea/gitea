// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/markup/common"

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

type globalVarsType struct {
	hashCurrentPattern      *regexp.Regexp
	shortLinkPattern        *regexp.Regexp
	anyHashPattern          *regexp.Regexp
	comparePattern          *regexp.Regexp
	fullURLPattern          *regexp.Regexp
	emailRegex              *regexp.Regexp
	blackfridayExtRegex     *regexp.Regexp
	emojiShortCodeRegex     *regexp.Regexp
	issueFullPattern        *regexp.Regexp
	filesChangedFullPattern *regexp.Regexp
	codePreviewPattern      *regexp.Regexp

	tagCleaner *regexp.Regexp
	nulCleaner *strings.Replacer
}

var globalVars = sync.OnceValue(func() *globalVarsType {
	v := &globalVarsType{}
	// NOTE: All below regex matching do not perform any extra validation.
	// Thus a link is produced even if the linked entity does not exist.
	// While fast, this is also incorrect and lead to false positives.
	// TODO: fix invalid linking issue

	// valid chars in encoded path and parameter: [-+~_%.a-zA-Z0-9/]

	// hashCurrentPattern matches string that represents a commit SHA, e.g. d8a994ef243349f321568f9e36d5c3f444b99cae
	// Although SHA1 hashes are 40 chars long, SHA256 are 64, the regex matches the hash from 7 to 64 chars in length
	// so that abbreviated hash links can be used as well. This matches git and GitHub usability.
	v.hashCurrentPattern = regexp.MustCompile(`(?:\s|^|\(|\[)([0-9a-f]{7,64})(?:\s|$|\)|\]|[.,:](\s|$))`)

	// shortLinkPattern matches short but difficult to parse [[name|link|arg=test]] syntax
	v.shortLinkPattern = regexp.MustCompile(`\[\[(.*?)\]\](\w*)`)

	// anyHashPattern splits url containing SHA into parts
	v.anyHashPattern = regexp.MustCompile(`https?://(?:\S+/){4,5}([0-9a-f]{40,64})(/[-+~%./\w]+)?(\?[-+~%.\w&=]+)?(#[-+~%.\w]+)?`)

	// comparePattern matches "http://domain/org/repo/compare/COMMIT1...COMMIT2#hash"
	v.comparePattern = regexp.MustCompile(`https?://(?:\S+/){4,5}([0-9a-f]{7,64})(\.\.\.?)([0-9a-f]{7,64})?(#[-+~_%.a-zA-Z0-9]+)?`)

	// fullURLPattern matches full URL like "mailto:...", "https://..." and "ssh+git://..."
	v.fullURLPattern = regexp.MustCompile(`^[a-z][-+\w]+:`)

	// emailRegex is definitely not perfect with edge cases,
	// it is still accepted by the CommonMark specification, as well as the HTML5 spec:
	//   http://spec.commonmark.org/0.28/#email-address
	//   https://html.spec.whatwg.org/multipage/input.html#e-mail-state-(type%3Demail)
	v.emailRegex = regexp.MustCompile("(?:\\s|^|\\(|\\[)([a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9]{2,}(?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+)(?:\\s|$|\\)|\\]|;|,|\\?|!|\\.(\\s|$))")

	// blackfridayExtRegex is for blackfriday extensions create IDs like fn:user-content-footnote
	v.blackfridayExtRegex = regexp.MustCompile(`[^:]*:user-content-`)

	// emojiShortCodeRegex find emoji by alias like :smile:
	v.emojiShortCodeRegex = regexp.MustCompile(`:[-+\w]+:`)

	// example: https://domain/org/repo/pulls/27#hash
	v.issueFullPattern = regexp.MustCompile(`https?://(?:\S+/)[\w_.-]+/[\w_.-]+/(?:issues|pulls)/((?:\w{1,10}-)?[1-9][0-9]*)([\?|#](\S+)?)?\b`)

	// example: https://domain/org/repo/pulls/27/files#hash
	v.filesChangedFullPattern = regexp.MustCompile(`https?://(?:\S+/)[\w_.-]+/[\w_.-]+/pulls/((?:\w{1,10}-)?[1-9][0-9]*)/files([\?|#](\S+)?)?\b`)

	// codePreviewPattern matches "http://domain/.../{owner}/{repo}/src/commit/{commit}/{filepath}#L10-L20"
	v.codePreviewPattern = regexp.MustCompile(`https?://\S+/([^\s/]+)/([^\s/]+)/src/commit/([0-9a-f]{7,64})(/\S+)#(L\d+(-L\d+)?)`)

	v.tagCleaner = regexp.MustCompile(`<((?:/?\w+/\w+)|(?:/[\w ]+/)|(/?[hH][tT][mM][lL]\b)|(/?[hH][eE][aA][dD]\b))`)
	v.nulCleaner = strings.NewReplacer("\000", "")
	return v
})

// IsFullURLBytes reports whether link fits valid format.
func IsFullURLBytes(link []byte) bool {
	return globalVars().fullURLPattern.Match(link)
}

func IsFullURLString(link string) bool {
	return globalVars().fullURLPattern.MatchString(link)
}

func IsNonEmptyRelativePath(link string) bool {
	return link != "" && !IsFullURLString(link) && link[0] != '/' && link[0] != '?' && link[0] != '#'
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
	common.GlobalVars().LinkRegex, _ = xurls.StrictMatchingScheme(strings.Join(withAuth, "|"))
}

type processor func(ctx *RenderContext, node *html.Node)

// PostProcessDefault does the final required transformations to the passed raw HTML
// data, and ensures its validity. Transformations include: replacing links and
// emails with HTML links, parsing shortlinks in the format of [[Link]], like
// MediaWiki, linking issues in the format #ID, and mentions in the format
// @user, and others.
func PostProcessDefault(ctx *RenderContext, input io.Reader, output io.Writer) error {
	procs := []processor{
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
	return postProcess(ctx, procs, input, output)
}

// PostProcessCommitMessage will use the same logic as PostProcess, but will disable
// the shortLinkProcessor.
func PostProcessCommitMessage(ctx *RenderContext, content string) (string, error) {
	procs := []processor{
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
	return postProcessString(ctx, procs, content)
}

var emojiProcessors = []processor{
	emojiShortCodeProcessor,
	emojiProcessor,
}

// PostProcessCommitMessageSubject will use the same logic as PostProcess and
// PostProcessCommitMessage, but will disable the shortLinkProcessor and
// emailAddressProcessor, will add a defaultLinkProcessor if defaultLink is set,
// which changes every text node into a link to the passed default link.
func PostProcessCommitMessageSubject(ctx *RenderContext, defaultLink, content string) (string, error) {
	procs := []processor{
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
	procs = append(procs, func(ctx *RenderContext, node *html.Node) {
		ch := &html.Node{Parent: node, Type: html.TextNode, Data: node.Data}
		node.Type = html.ElementNode
		node.Data = "a"
		node.DataAtom = atom.A
		node.Attr = []html.Attribute{{Key: "href", Val: defaultLink}, {Key: "class", Val: "muted"}}
		node.FirstChild, node.LastChild = ch, ch
	})
	return postProcessString(ctx, procs, content)
}

// PostProcessIssueTitle to process title on individual issue/pull page
func PostProcessIssueTitle(ctx *RenderContext, title string) (string, error) {
	return postProcessString(ctx, []processor{
		issueIndexPatternProcessor,
		commitCrossReferencePatternProcessor,
		hashCurrentPatternProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, title)
}

// PostProcessDescriptionHTML will use similar logic as PostProcess, but will
// use a single special linkProcessor.
func PostProcessDescriptionHTML(ctx *RenderContext, content string) (string, error) {
	return postProcessString(ctx, []processor{
		descriptionLinkProcessor,
		emojiShortCodeProcessor,
		emojiProcessor,
	}, content)
}

// PostProcessEmoji for when we want to just process emoji and shortcodes
// in various places it isn't already run through the normal markdown processor
func PostProcessEmoji(ctx *RenderContext, content string) (string, error) {
	return postProcessString(ctx, emojiProcessors, content)
}

func postProcessString(ctx *RenderContext, procs []processor, content string) (string, error) {
	var buf strings.Builder
	if err := postProcess(ctx, procs, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func postProcess(ctx *RenderContext, procs []processor, input io.Reader, output io.Writer) error {
	if !ctx.usedByRender && ctx.RenderHelper != nil {
		defer ctx.RenderHelper.CleanUp()
	}
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
		bytes.NewReader(globalVars().tagCleaner.ReplaceAll([]byte(globalVars().nulCleaner.Replace(string(rawHTML))), []byte("&lt;$1"))),
		// close the tags
		strings.NewReader("</body></html>"),
	))
	if err != nil {
		return fmt.Errorf("markup.postProcess: invalid HTML: %w", err)
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
			return fmt.Errorf("markup.postProcess: html.Render: %w", err)
		}
	}
	return nil
}

func isEmojiNode(node *html.Node) bool {
	if node.Type == html.ElementNode && node.Data == atom.Span.String() {
		for _, attr := range node.Attr {
			if (attr.Key == "class" || attr.Key == "data-attr-class") && strings.Contains(attr.Val, "emoji") {
				return true
			}
		}
	}
	return false
}

func visitNode(ctx *RenderContext, procs []processor, node *html.Node) *html.Node {
	// Add user-content- to IDs and "#" links if they don't already have them
	for idx, attr := range node.Attr {
		val := strings.TrimPrefix(attr.Val, "#")
		notHasPrefix := !(strings.HasPrefix(val, "user-content-") || globalVars().blackfridayExtRegex.MatchString(val))

		if attr.Key == "id" && notHasPrefix {
			node.Attr[idx].Val = "user-content-" + attr.Val
		}

		if attr.Key == "href" && strings.HasPrefix(attr.Val, "#") && notHasPrefix {
			node.Attr[idx].Val = "#user-content-" + val
		}
	}

	switch node.Type {
	case html.TextNode:
		for _, proc := range procs {
			proc(ctx, node) // it might add siblings
		}

	case html.ElementNode:
		if isEmojiNode(node) {
			// TextNode emoji will be converted to `<span class="emoji">`, then the next iteration will visit the "span"
			// if we don't stop it, it will go into the TextNode again and create an infinite recursion
			return node.NextSibling
		} else if node.Data == "code" || node.Data == "pre" {
			return node.NextSibling // ignore code and pre nodes
		} else if node.Data == "img" {
			return visitNodeImg(ctx, node)
		} else if node.Data == "video" {
			return visitNodeVideo(ctx, node)
		} else if node.Data == "a" {
			procs = emojiProcessors // Restrict text in links to emojis
		}
		for n := node.FirstChild; n != nil; {
			n = visitNode(ctx, procs, n)
		}
	default:
	}
	return node.NextSibling
}

// createKeyword() renders a highlighted version of an action keyword
func createKeyword(ctx *RenderContext, content string) *html.Node {
	// CSS class for action keywords (e.g. "closes: #1")
	const keywordClass = "issue-keyword"

	span := &html.Node{
		Type: html.ElementNode,
		Data: atom.Span.String(),
		Attr: []html.Attribute{},
	}
	span.Attr = append(span.Attr, ctx.RenderInternal.NodeSafeAttr("class", keywordClass))

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}
	span.AppendChild(text)

	return span
}

func createLink(ctx *RenderContext, href, content, class string) *html.Node {
	a := &html.Node{
		Type: html.ElementNode,
		Data: atom.A.String(),
		Attr: []html.Attribute{{Key: "href", Val: href}},
	}
	if !RenderBehaviorForTesting.DisableAdditionalAttributes {
		a.Attr = append(a.Attr, html.Attribute{Key: "data-markdown-generated-content"})
	}
	if class != "" {
		a.Attr = append(a.Attr, ctx.RenderInternal.NodeSafeAttr("class", class))
	}

	text := &html.Node{
		Type: html.TextNode,
		Data: content,
	}

	a.AppendChild(text)
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
