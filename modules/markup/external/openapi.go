// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"fmt"
	"html"
	"io"

	"code.gitea.io/gitea/modules/markup"
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
	ret.ContentSandbox = ""
	return ret
}

func (p *openAPIRenderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	content, err := util.ReadWithLimit(input, int(setting.UI.MaxDisplayFileSize))
	if err != nil {
		return err
	}
	// TODO: can extract this to a tmpl file later
	_, err = io.WriteString(output, fmt.Sprintf(
		`<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<link rel="stylesheet" href="%s/assets/css/swagger.css?v=%s">
</head>
<body>
	<div id="swagger-ui"><textarea class="swagger-spec-content" data-spec-filename="%s">%s</textarea></div>
	<script src="%s/assets/js/swagger.js?v=%s"></script>
</body>
</html>`,
		setting.StaticURLPrefix,
		setting.AssetVersion,
		html.EscapeString(ctx.RenderOptions.RelativePath),
		html.EscapeString(util.UnsafeBytesToString(content)),
		setting.StaticURLPrefix,
		setting.AssetVersion,
	))
	return err
}
