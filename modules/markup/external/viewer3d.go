// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package external

import (
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"path"

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
	ret.ContentSandbox = "allow-scripts"
	return ret
}

func (p *viewer3dRenderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	if ctx.RenderOptions.StandalonePageOptions == nil {
		opts := p.GetExternalRendererOptions()
		return markup.RenderIFrame(ctx, &opts, output)
	}

	if _, err := fmt.Fprintf(output,
		`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><style>html,body{margin:0;height:100%%}#viewer{width:100%%;height:100%%}#viewer>div{text-align:center;padding:1em}</style></head>
<body><div id="viewer"></div><script type="application/octet-stream" id="modelData" data-filename="%s">`,
		html.EscapeString(path.Base(ctx.RenderOptions.RelativePath)),
	); err != nil {
		return err
	}

	b64 := base64.NewEncoder(base64.StdEncoding, output)
	if _, err := io.Copy(b64, io.LimitReader(input, setting.UI.MaxDisplayFileSize)); err != nil {
		return err
	}
	if err := b64.Close(); err != nil {
		return err
	}

	_, err := fmt.Fprintf(output, `</script><script type="module" src="%s"></script></body></html>`, public.AssetURI("js/viewer-3d.js"))
	return err
}
