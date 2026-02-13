// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"io"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
)

// Renderer defines an interface for rendering markup file to HTML
type Renderer interface {
	Name() string // markup format name, also the renderer type, also the external tool name
	FileNamePatterns() []string
	SanitizerRules() []setting.MarkupSanitizerRule
	Render(ctx *RenderContext, input io.Reader, output io.Writer) error
}

// PostProcessRenderer defines an interface for renderers who need post process
type PostProcessRenderer interface {
	NeedPostProcess() bool
}

type ExternalRendererOptions struct {
	SanitizerDisabled bool
	DisplayInIframe   bool
	ContentSandbox    string
}

// ExternalRenderer defines an interface for external renderers
type ExternalRenderer interface {
	GetExternalRendererOptions() ExternalRendererOptions
}

// RendererContentDetector detects if the content can be rendered
// by specified renderer
type RendererContentDetector interface {
	CanRender(filename string, sniffedType typesniffer.SniffedType, prefetchBuf []byte) bool
}

var (
	fileNameRenderers = make(map[string]Renderer)
	renderers         = make(map[string]Renderer)
)

// RegisterRenderer registers a new markup file renderer
func RegisterRenderer(renderer Renderer) {
	// TODO: need to handle conflicts
	renderers[renderer.Name()] = renderer
}

func RefreshFileNamePatterns() {
	// TODO: need to handle conflicts
	fileNameRenderers = make(map[string]Renderer)
	for _, renderer := range renderers {
		for _, ext := range renderer.FileNamePatterns() {
			fileNameRenderers[strings.ToLower(ext)] = renderer
		}
	}
}

func DetectRendererTypeByFilename(filename string) Renderer {
	basename := path.Base(strings.ToLower(filename))
	ext1 := path.Ext(basename)
	if renderer := fileNameRenderers[basename]; renderer != nil {
		return renderer
	}
	if renderer := fileNameRenderers["*"+ext1]; renderer != nil {
		return renderer
	}
	if basename, ok := strings.CutSuffix(basename, ext1); ok {
		ext2 := path.Ext(basename)
		if renderer := fileNameRenderers["*"+ext2+ext1]; renderer != nil {
			return renderer
		}
	}
	return nil
}

// DetectRendererTypeByPrefetch detects the markup type of the content
func DetectRendererTypeByPrefetch(filename string, sniffedType typesniffer.SniffedType, prefetchBuf []byte) string {
	if filename != "" {
		byExt := DetectRendererTypeByFilename(filename)
		if byExt != nil {
			return byExt.Name()
		}
	}
	for _, renderer := range renderers {
		if detector, ok := renderer.(RendererContentDetector); ok && detector.CanRender(filename, sniffedType, prefetchBuf) {
			return renderer.Name()
		}
	}
	return ""
}

func PreviewableExtensions() []string {
	exts := make([]string, 0, len(fileNameRenderers))
	for p := range fileNameRenderers {
		if s, ok := strings.CutPrefix(p, "*"); ok {
			exts = append(exts, s)
		}
	}
	return exts
}
