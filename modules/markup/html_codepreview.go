// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"

	"golang.org/x/net/html"
)

type RenderCodePreviewOptions struct {
	FullURL   string
	OwnerName string
	RepoName  string
	CommitID  string
	FilePath  string

	LineStart, LineStop int
}

func renderCodeBlock(ctx *RenderContext, node *html.Node) (urlPosStart, urlPosStop int, htm template.HTML) {
	start := 0
	for {
		data := node.Data[start:]
		m := globalVars().codePreviewPattern.FindStringSubmatchIndex(data)
		if m == nil {
			return 0, 0, ""
		}
		urlPosStart, urlPosStop = start+m[0], start+m[1]
		start = urlPosStop

		opts := RenderCodePreviewOptions{
			FullURL:   data[m[0]:m[1]],
			OwnerName: data[m[2]:m[3]],
			RepoName:  data[m[4]:m[5]],
			CommitID:  data[m[6]:m[7]],
			FilePath:  data[m[8]:m[9]],
		}
		if !httplib.IsCurrentGiteaSiteURL(ctx, opts.FullURL) {
			continue
		}
		u, err := url.Parse(opts.FilePath)
		if err != nil {
			log.Error("Unable to render code preview: %v", err)
			continue
		}
		opts.FilePath = strings.TrimPrefix(u.Path, "/")

		lineStartStr, lineStopStr, _ := strings.Cut(data[m[10]:m[11]], "-")
		lineStart, _ := strconv.Atoi(strings.TrimPrefix(lineStartStr, "L"))
		lineStop, _ := strconv.Atoi(strings.TrimPrefix(lineStopStr, "L"))
		opts.LineStart, opts.LineStop = lineStart, lineStop
		h, err := DefaultRenderHelperFuncs.RenderRepoFileCodePreview(ctx, opts)
		if err != nil {
			log.Error("Unable to render code preview: %v", err)
		} else if h != "" {
			return urlPosStart, urlPosStop, h
		}
	}
}

func codePreviewPatternProcessor(ctx *RenderContext, node *html.Node) {
	if ctx.RenderOptions.FeedExcerpt {
		return
	}
	nodeStop := node.NextSibling
	for node != nodeStop {
		if node.Type != html.TextNode {
			node = node.NextSibling
			continue
		}
		urlPosStart, urlPosEnd, renderedCodeBlock := renderCodeBlock(ctx, node)
		if renderedCodeBlock == "" {
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
		renderedCodeNode := &html.Node{Type: html.RawNode, Data: string(ctx.RenderInternal.ProtectSafeAttrs(renderedCodeBlock))}
		node.Parent.InsertBefore(renderedCodeNode, next)
		if textAfter != "" {
			node.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: textAfter}, next)
		}
		node = next
	}
}
