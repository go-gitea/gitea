// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"encoding/base64"
	"io"
	"unicode/utf8"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type frontendRenderer struct {
	name     string
	patterns []string
}

var (
	_ markup.PostProcessRenderer = (*frontendRenderer)(nil)
	_ markup.ExternalRenderer    = (*frontendRenderer)(nil)
)

func (p *frontendRenderer) Name() string {
	return p.name
}

func (p *frontendRenderer) NeedPostProcess() bool {
	return false
}

func (p *frontendRenderer) FileNamePatterns() []string {
	// TODO: the file extensions are ambiguous, even if the file name matches, it doesn't mean that the file is a 3D model
	// There are some approaches to make it more accurate, but they are all complicated:
	// A. Make backend know everything (detect a file is a 3D model or not)
	// B. Let frontend renders to try render one by one
	//
	// If there would be more frontend renders in the future, we need to implement the "frontend" approach:
	// 1. Make backend or parent window collect the supported extensions of frontend renders (done: backend external render framework)
	// 2. If the current file matches any extension, start the general iframe embedded render (done: this renderer)
	// 3. The iframe window calls the frontend renders one by one (done: frontend external render)
	// 4. Report the render result to parent by postMessage (TODO: when needed)
	return p.patterns
}

func (p *frontendRenderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return nil
}

func (p *frontendRenderer) GetExternalRendererOptions() (ret markup.ExternalRendererOptions) {
	ret.SanitizerDisabled = true
	ret.DisplayInIframe = true
	ret.ContentSandbox = "allow-scripts allow-forms allow-modals allow-popups allow-downloads"
	return ret
}

func (p *frontendRenderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.RenderOptions.StandalonePageOptions == nil {
		opts := p.GetExternalRendererOptions()
		return markup.RenderIFrame(ctx, &opts, output)
	}

	content, err := util.ReadWithLimit(input, int(setting.UI.MaxDisplayFileSize))
	if err != nil {
		return err
	}

	contentEncoding, contentString := "text", util.UnsafeBytesToString(content)
	if !utf8.Valid(content) {
		contentEncoding = "base64"
		contentString = base64.StdEncoding.EncodeToString(content)
	}

	_, err = htmlutil.HTMLPrintf(output,
		`<!DOCTYPE html>
<html>
<head>
	<!-- external-render-helper will be injected here by the markup render -->
	<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
	<div id="frontend-render-viewer" data-frontend-renders="%s" data-file-tree-path="%s"></div>
	<textarea id="frontend-render-data" data-content-encoding="%s" hidden>%s</textarea>
	<script nonce type="module" src="%s"></script>
</body>
</html>`,
		p.name, ctx.RenderOptions.RelativePath,
		contentEncoding, contentString,
		public.AssetURI("js/external-render-frontend.js"))
	return err
}
