// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gitea.dev/modules/base"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/markup/common"
	"gitea.dev/modules/references"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type anyHashPatternResult struct {
	PosStart    int
	PosEnd      int
	FullURL     string
	CommitID    string
	CommitExt   string
	SubPath     string
	QueryParams string
	QueryHash   string
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

func anyHashPatternExtract(s string) (ret anyHashPatternResult, ok bool) {
	m := globalVars().anyHashPattern.FindStringSubmatchIndex(s)
	if m == nil {
		return ret, false
	}

	pos := 0

	ret.PosStart, ret.PosEnd = m[pos], m[pos+1]
	pos += 2

	ret.FullURL = s[ret.PosStart:ret.PosEnd]
	if strings.HasSuffix(ret.FullURL, ".") {
		// if url ends in '.', it's very likely that it is not part of the actual url but used to finish a sentence.
		ret.PosEnd--
		ret.FullURL = ret.FullURL[:len(ret.FullURL)-1]
		for i := range m {
			m[i] = min(m[i], ret.PosEnd)
		}
	}

	ret.CommitID = s[m[pos]:m[pos+1]]
	pos += 2

	ret.CommitExt = s[m[pos]:m[pos+1]]
	pos += 4

	if m[pos] > 0 {
		ret.SubPath = s[m[pos]:m[pos+1]]
	}
	pos += 2

	if m[pos] > 0 {
		ret.QueryParams = s[m[pos]:m[pos+1]]
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
		ret, ok := anyHashPatternExtract(node.Data)
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
		// only turn commit links to the current instance into hash link
		if !httplib.IsCurrentGiteaSiteURL(ctx, ret.FullURL) {
			node = node.NextSibling
			continue
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

		// only turn compare links to the current instance into hash link
		if !httplib.IsCurrentGiteaSiteURL(ctx, urlFull) {
			node = node.NextSibling
			continue
		}

		text := text1 + textDots + text2
		if hash != "" {
			text += " (" + hash + ")"
		}
		replaceContent(node, start, end, createCodeLink(urlFull, text, "compare"))
		node = node.NextSibling.NextSibling
	}
}

// hashCurrentPatternProcessor links commit IDs and "A...B" ranges of the current repository by their full commit IDs
func hashCurrentPatternProcessor(ctx *RenderContext, node *html.Node) {
	metas := ctx.RenderOptions.Metas
	if metas == nil || metas["user"] == "" || metas["repo"] == "" || ctx.RenderHelper == nil {
		return
	}
	repoLink := "/:root/" + metas["user"] + "/" + metas["repo"]

	start := 0
	next := node.NextSibling
	for node != nil && node != next && start < len(node.Data) {
		m := globalVars().hashCurrentPattern.FindStringSubmatchIndex(node.Data[start:])
		if m == nil {
			return
		}
		for i := range m {
			if m[i] >= 0 {
				m[i] += start
			}
		}
		end := max(m[3], m[5]) // not m[1], the consumed boundary char may lead the next match
		link := createHashLink(ctx, repoLink, node.Data, m)
		if link == nil {
			start = end
			continue
		}
		replaceContent(node, m[2], end, link)
		start = 0
		node = node.NextSibling.NextSibling
	}
}

// createHashLink returns nil if a matched ID doesn't resolve or the match sits inside a URL or email that a later processor links
func createHashLink(ctx *RenderContext, repoLink, text string, m []int) *html.Node {
	if isInLinkOrEmail(text, m[2]) {
		return nil
	}
	fullID := ctx.RenderHelper.ResolveCommitID(text[m[2]:m[3]])
	if fullID == "" {
		return nil
	}
	if m[4] < 0 {
		return createCodeLink(repoLink+"/commit/"+fullID, base.ShortSha(fullID), "commit")
	}
	fullID2 := ctx.RenderHelper.ResolveCommitID(text[m[4]:m[5]])
	if fullID2 == "" {
		return nil
	}
	return createCodeLink(repoLink+"/compare/"+fullID+"..."+fullID2, base.ShortSha(fullID)+"..."+base.ShortSha(fullID2), "compare")
}

func isInLinkOrEmail(text string, pos int) bool {
	for _, re := range []*regexp.Regexp{common.GlobalVars().LinkifyRegex, globalVars().emailRegex} {
		for _, loc := range re.FindAllStringIndex(text, -1) {
			if loc[0] <= pos && pos < loc[1] {
				return true
			}
		}
	}
	return false
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
