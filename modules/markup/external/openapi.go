// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"fmt"
	"html"
	"io"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type openAPIRenderer struct{}

var (
	_ markup.PostProcessRenderer = (*openAPIRenderer)(nil)
	_ markup.ExternalRenderer    = (*openAPIRenderer)(nil)
)

func (p *openAPIRenderer) Name() string {
	return "openapi"
}

func (p *openAPIRenderer) NeedPostProcess() bool {
	return false
}

func (p *openAPIRenderer) FileNamePatterns() []string {
	return []string{
		"openapi.yaml",
		"openapi.yml",
		"openapi.json",
		"swagger.yaml",
		"swagger.yml",
		"swagger.json",
	}
}

func (p *openAPIRenderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return nil
}

func (p *openAPIRenderer) GetExternalRendererOptions() (ret markup.ExternalRendererOptions) {
	ret.SanitizerDisabled = true
	ret.DisplayInIframe = true
	ret.ContentSandbox = "allow-scripts allow-forms allow-modals allow-popups allow-downloads"
	return ret
}

func (p *openAPIRenderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.RenderOptions.StandalonePageOptions == nil {
		opts := p.GetExternalRendererOptions()
		return markup.RenderIFrame(ctx, &opts, output)
	}

	content, err := util.ReadWithLimit(input, int(setting.UI.MaxDisplayFileSize))
	if err != nil {
		return err
	}

	// HINT: SWAGGER-OPENAPI-VIEWER: another place "templates/swagger/openapi-viewer.tmpl"
	_, err = io.WriteString(output, fmt.Sprintf(
		`<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="stylesheet" href="%s">
</head>
<body>
	<div id="swagger-ui"><textarea class="swagger-spec-content" data-spec-filename="%s">%s</textarea></div>
	<script nonce type="module" src="%s"></script>
</body>
</html>`,
		public.AssetURI("css/swagger.css"),
		html.EscapeString(ctx.RenderOptions.RelativePath),
		html.EscapeString(util.UnsafeBytesToString(content)),
		public.AssetURI("js/swagger.js"),
	))
	return err
}
