// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/references"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type anyHashPatternResult struct {
	PosStart  int
	PosEnd    int
	FullURL   string
	CommitID  string
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
		Attr: []html.Attribute{{Key: "class", Val: "nohighlight"}},
	}

	code.AppendChild(text)
	a.AppendChild(code)
	return a
}

func anyHashPatternExtract(s string) (ret anyHashPatternResult, ok bool) {
	m := globalVars().anyHashPattern.FindStringSubmatchIndex(s)
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
	if ctx.RenderOptions.Metas == nil {
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
	if ctx.RenderOptions.Metas == nil {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		m := globalVars().comparePattern.FindStringSubmatchIndex(node.Data)
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

		link := ctx.RenderHelper.ResolveLink(util.URLJoin(ctx.RenderOptions.Metas["user"], ctx.RenderOptions.Metas["repo"], "commit", hash), LinkTypeApp)
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

		reftext := ref.Owner + "/" + ref.Name + "@" + base.ShortSha(ref.CommitSha)
		linkHref := ctx.RenderHelper.ResolveLink(util.URLJoin(ref.Owner, ref.Name, "commit", ref.CommitSha), LinkTypeApp)
		link := createLink(ctx, linkHref, reftext, "commit")

		replaceContent(node, ref.RefLocation.Start, ref.RefLocation.End, link)
		node = node.NextSibling.NextSibling
	}
}
