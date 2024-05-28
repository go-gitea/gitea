// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"html/template"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"

	"golang.org/x/net/html"
)

// codePreviewPattern matches "http://domain/.../{owner}/{repo}/src/commit/{commit}/{filepath}#L10-L20"
var codePreviewPattern = regexp.MustCompile(`https?://\S+/([^\s/]+)/([^\s/]+)/src/commit/([0-9a-f]{7,64})(/\S+)#(L\d+(-L\d+)?)`)

type RenderCodePreviewOptions struct {
	FullURL   string
	OwnerName string
	RepoName  string
	CommitID  string
	FilePath  string

	LineStart, LineStop int
}

func renderCodeBlock(ctx *RenderContext, node *html.Node) (urlPosStart, urlPosStop int, htm template.HTML, err error) {
	m := codePreviewPattern.FindStringSubmatchIndex(node.Data)
	if m == nil {
		return 0, 0, "", nil
	}

	opts := RenderCodePreviewOptions{
		FullURL:   node.Data[m[0]:m[1]],
		OwnerName: node.Data[m[2]:m[3]],
		RepoName:  node.Data[m[4]:m[5]],
		CommitID:  node.Data[m[6]:m[7]],
		FilePath:  node.Data[m[8]:m[9]],
	}
	if !httplib.IsCurrentGiteaSiteURL(ctx.Ctx, opts.FullURL) {
		return 0, 0, "", nil
	}
	u, err := url.Parse(opts.FilePath)
	if err != nil {
		return 0, 0, "", err
	}
	opts.FilePath = strings.TrimPrefix(u.Path, "/")

	lineStartStr, lineStopStr, _ := strings.Cut(node.Data[m[10]:m[11]], "-")
	lineStart, _ := strconv.Atoi(strings.TrimPrefix(lineStartStr, "L"))
	lineStop, _ := strconv.Atoi(strings.TrimPrefix(lineStopStr, "L"))
	opts.LineStart, opts.LineStop = lineStart, lineStop
	h, err := DefaultProcessorHelper.RenderRepoFileCodePreview(ctx.Ctx, opts)
	return m[0], m[1], h, err
}

func codePreviewPatternProcessor(ctx *RenderContext, node *html.Node) {
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		urlPosStart, urlPosEnd, h, err := renderCodeBlock(ctx, node)
		if err != nil || h == "" {
			if err != nil {
				log.Error("Unable to render code preview: %v", err)
			}
			node = node.NextSibling
			continue
		}
		next := node.NextSibling
		textBefore := node.Data[:urlPosStart]
		textAfter := node.Data[urlPosEnd:]
		// "textBefore" could be empty if there is only a URL in the text node, then an empty node (p, or li) will be left here.
		// However, the empty node can't be simply removed, because:
		// 1. the following processors will still try to access it (need to double-check undefined behaviors)
		// 2. the new node is inserted as "<p>{TextBefore}<div NewNode/>{TextAfter}</p>" (the parent could also be "li")
		//    then it is resolved as: "<p>{TextBefore}</p><div NewNode/><p>{TextAfter}</p>",
		//    so unless it could correctly replace the parent "p/li" node, it is very difficult to eliminate the "TextBefore" empty node.
		node.Data = textBefore
		node.Parent.InsertBefore(&html.Node{Type: html.RawNode, Data: string(h)}, next)
		if textAfter != "" {
			node.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: textAfter}, next)
		}
		node = next
	}
}
