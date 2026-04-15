// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"encoding/base64"
	"io"
	"path"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
)

type viewer3dRenderer struct{}

var (
	_ markup.PostProcessRenderer = (*viewer3dRenderer)(nil)
	_ markup.ExternalRenderer    = (*viewer3dRenderer)(nil)
)

func (p *viewer3dRenderer) Name() string {
	return "viewer3d"
}

func (p *viewer3dRenderer) NeedPostProcess() bool {
	return false
}

func (p *viewer3dRenderer) FileNamePatterns() []string {
	// TODO: the file extensions are ambiguous, even if the file name matches, it doesn't mean that the file is a 3D model
	// There are some approaches to make it more accurate, but they are all complicated:
	// A. Make backend know everything (detect a file is a 3D model or not)
	// B. Let frontend renders to try render one by one
	//
	// If there would be more frontend renders in the future, we need to implement the "frontend" approach:
	// 1. Make parent window collect the supported extensions of frontend renders
	// 2. If the current file matches any extension, start the general iframe embedded render
	// 3. The iframe window calls the frontend renders one by one, and report the render result to parent by postMessage
	return []string{
		"*.3dm", "*.3ds", "*.3mf", "*.amf", "*.bim", "*.brep",
		"*.dae", "*.fbx", "*.fcstd", "*.glb", "*.gltf",
		"*.ifc", "*.igs", "*.iges", "*.stp", "*.step",
		"*.stl", "*.obj", "*.off", "*.ply", "*.wrl",
	}
}

func (p *viewer3dRenderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return nil
}

func (p *viewer3dRenderer) GetExternalRendererOptions() (ret markup.ExternalRendererOptions) {
	ret.SanitizerDisabled = true
	ret.DisplayInIframe = true
	ret.ContentSandbox = "allow-scripts allow-forms allow-modals allow-popups allow-downloads"
	return ret
}

func (p *viewer3dRenderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.RenderOptions.StandalonePageOptions == nil {
		opts := p.GetExternalRendererOptions()
		return markup.RenderIFrame(ctx, &opts, output)
	}

	if _, err := htmlutil.HTMLPrintf(output,
		`<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<style>
		html,body{margin:0;height:100%%}
		#viewer{width:100%%;height:100%%}
		#viewer>div{text-align:center;padding:1em}
	</style>
</head>
<body>
	<div id="viewer"></div>
	<script type="application/octet-stream" id="modelData" data-filename="%s">`,
		path.Base(ctx.RenderOptions.RelativePath),
	); err != nil {
		return err
	}

	b64 := base64.NewEncoder(base64.StdEncoding, output)
	defer b64.Close()
	_, err := io.Copy(b64, io.LimitReader(input, setting.UI.MaxDisplayFileSize))
	if err != nil {
		return err
	}
	_ = b64.Close()

	_, err = htmlutil.HTMLPrintf(output, `</script><script type="module" src="%s"></script></body></html>`, public.AssetURI("js/viewer-3d.js"))
	return err
}
