// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/references"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type anyHashPatternResult struct {
	PosStart  int
	PosEnd    int
	FullURL   string
	CommitID  string
	CommitExt string
	SubPath   string
	QueryHash string
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
	}

	code.AppendChild(text)
	a.AppendChild(code)
	return a
}

// stripTrailingSentencePeriod trims a trailing '.' that is likely sentence punctuation rather than part of the URL.
// It also clamps capture-group indices in m in place so they don't point past the trimmed URL.
func stripTrailingSentencePeriod(fullURL string, posEnd int, m []int) (string, int) {
	if !strings.HasSuffix(fullURL, ".") {
		return fullURL, posEnd
	}
	posEnd--
	fullURL = fullURL[:len(fullURL)-1]
	for i := range m {
		m[i] = min(m[i], posEnd)
	}
	return fullURL, posEnd
}

// isRepoCommitRoutePath reports whether path (a Gitea repo subpath) identifies a commit by hash.
// It accepts `/commit/...`, `/archive/...`, or `/<group>/commit/...` (Gitea's RefTypeCommit route shape).
func isRepoCommitRoutePath(path string) bool {
	if strings.HasPrefix(path, "/commit/") || strings.HasPrefix(path, "/archive/") {
		return true
	}
	_, rest, ok := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	return ok && strings.HasPrefix(rest, "commit/")
}

func anyHashPatternExtract(ctx context.Context, s string) (ret anyHashPatternResult, ok bool) {
	m := globalVars().anyHashPattern.FindStringSubmatchIndex(s)
	if m == nil {
		return ret, false
	}

	pos := 0

	ret.PosStart, ret.PosEnd = m[pos], m[pos+1]
	pos += 2

	ret.FullURL = s[ret.PosStart:ret.PosEnd]
	ret.FullURL, ret.PosEnd = stripTrailingSentencePeriod(ret.FullURL, ret.PosEnd, m)

	// reject URLs outside this Gitea instance or not shaped as a repo commit-route path
	parsed := httplib.ParseGiteaSiteURL(ctx, ret.FullURL)
	if parsed == nil || !isRepoCommitRoutePath(parsed.RepoSubPath) {
		return ret, false
	}

	ret.CommitID = s[m[pos]:m[pos+1]]
	pos += 2

	ret.CommitExt = s[m[pos]:m[pos+1]]
	pos += 2

	if m[pos] > 0 {
		ret.SubPath = s[m[pos]:m[pos+1]]
	}
	pos += 2

	if m[pos] > 0 {
		ret.QueryHash = s[m[pos]:m[pos+1]][1:]
	}
	return ret, true
}

// fullHashPatternProcessor renders SHA containing URLs
func fullHashPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		ret, ok := anyHashPatternExtract(ctx, node.Data)
		if !ok {
			node = node.NextSibling
			continue
		}
		text := base.ShortSha(ret.CommitID)
		if ret.CommitExt != "" {
			text += ret.CommitExt
		}
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

type comparePatternResult struct {
	PosStart int
	PosEnd   int
	FullURL  string
	Hash1    string
	Dots     string
	Hash2    string
	Fragment string
}

func comparePatternExtract(ctx context.Context, s string) (ret comparePatternResult, ok bool) {
	m := globalVars().comparePattern.FindStringSubmatchIndex(s)
	if m == nil || slices.Contains(m[:8], -1) { // full match + hash1 + dots + hash2 all required
		return ret, false
	}

	ret.PosStart, ret.PosEnd = m[0], m[1]
	ret.FullURL = s[ret.PosStart:ret.PosEnd]
	ret.FullURL, ret.PosEnd = stripTrailingSentencePeriod(ret.FullURL, ret.PosEnd, m)

	// reject URLs outside this Gitea instance or not shaped as /{owner}/{repo}/compare/...
	parsed := httplib.ParseGiteaSiteURL(ctx, ret.FullURL)
	if parsed == nil || !strings.HasPrefix(parsed.RepoSubPath, "/compare/") {
		return ret, false
	}

	ret.Hash1 = s[m[2]:m[3]]
	ret.Dots = s[m[4]:m[5]]
	ret.Hash2 = s[m[6]:m[7]]
	if m[9] > 0 {
		ret.Fragment = s[m[8]:m[9]][1:]
	}
	return ret, true
}

func comparePatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		ret, ok := comparePatternExtract(ctx, node.Data)
		if !ok {
			node = node.NextSibling
			continue
		}
		text := base.ShortSha(ret.Hash1) + ret.Dots + base.ShortSha(ret.Hash2)
		if ret.Fragment != "" {
			text += " (" + ret.Fragment + ")"
		}
		replaceContent(node, ret.PosStart, ret.PosEnd, createCodeLink(ret.FullURL, text, "compare"))
		node = node.NextSibling.NextSibling
	}
}

// hashCurrentPatternProcessor renders SHA1 strings to corresponding links that
// are assumed to be in the same repository.
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.Metas == nil || ctx.RenderOptions.Metas["user"] == "" || ctx.RenderOptions.Metas["repo"] == "" || ctx.RenderHelper == nil {
		return
	}

	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := globalVars().hashCurrentPattern.FindStringSubmatchIndex(node.Data[start:])
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
		if !ctx.RenderHelper.IsCommitIDExisting(hash) {
			start = m[3]
			continue
		}

		link := fmt.Sprintf("/:root/%s/%s/commit/%s", ctx.RenderOptions.Metas["user"], ctx.RenderOptions.Metas["repo"], hash)
		replaceContent(node, m[2], m[3], createCodeLink(link, base.ShortSha(hash), "commit"))
		start = 0
		node = node.NextSibling.NextSibling
	}
}

func commitCrossReferencePatternProcessor(ctx *RenderContext, node *html.Node) {
	next := node.NextSibling

	for node != nil && node != next {
		found, ref := references.FindRenderizableCommitCrossReference(node.Data)
		if !found {
			return
		}

		refText := ref.Owner + "/" + ref.Name + "@" + base.ShortSha(ref.CommitSha)
		linkHref := fmt.Sprintf("/:root/%s/%s/commit/%s", ref.Owner, ref.Name, ref.CommitSha)
		link := createLink(ctx, linkHref, refText, "commit")

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}
