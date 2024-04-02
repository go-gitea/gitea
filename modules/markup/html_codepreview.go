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
	if !httplib.IsCurrentGiteaSiteURL(opts.FullURL) {
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
	for node != nil {
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
		node.Data = textBefore
		node.Parent.InsertBefore(&html.Node{Type: html.RawNode, Data: string(h)}, next)
		if textAfter != "" {
			node.Parent.InsertBefore(&html.Node{Type: html.TextNode, Data: textAfter}, next)
		}
		node = next
	}
}
