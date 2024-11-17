// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
)

type RenderMetaMode string

const (
	RenderMetaAsDetails RenderMetaMode = "details" // default
	RenderMetaAsNone    RenderMetaMode = "none"
	RenderMetaAsTable   RenderMetaMode = "table"
)

var RenderBehaviorForTesting struct {
	// Markdown line break rendering has 2 default behaviors:
	// * Use hard: replace "\n" with "<br>" for comments, setting.Markdown.EnableHardLineBreakInComments=true
	// * Keep soft: "\n" for non-comments (a.k.a. documents), setting.Markdown.EnableHardLineBreakInDocuments=false
	// In history, there was a mess:
	// * The behavior was controlled by `Metas["mode"] != "document",
	// * However, many places render the content without setting "mode" in Metas, all these places used comment line break setting incorrectly
	ForceHardLineBreak bool

	// Gitea will emit some internal attributes for various purposes, these attributes don't affect rendering.
	// But there are too many hard-coded test cases, to avoid changing all of them again and again, we can disable emitting these internal attributes.
	DisableInternalAttributes bool
}

// RenderContext represents a render context
type RenderContext struct {
	Ctx          context.Context
	RelativePath string // relative path from tree root of the branch

	// eg: "orgmode", "asciicast", "console"
	// for file mode, it could be left as empty, and will be detected by file extension in RelativePath
	MarkupType string

	Links Links // special link references for rendering, especially when there is a branch/tree path

	// user&repo, format&style&regexp (for external issue pattern), teams&org (for mention)
	// BranchNameSubURL (for iframe&asciicast)
	// markupAllowShortIssuePattern, markupContentMode (wiki)
	// markdownLineBreakStyle (comment, document)
	Metas map[string]string

	GitRepo          *git.Repository
	Repo             gitrepo.Repository
	ShaExistCache    map[string]bool
	cancelFn         func()
	SidebarTocNode   ast.Node
	RenderMetaAs     RenderMetaMode
	InStandalonePage bool // used by external render. the router "/org/repo/render/..." will output the rendered content in a standalone page
}

// Cancel runs any cleanup functions that have been registered for this Ctx
func (ctx *RenderContext) Cancel() {
	if ctx == nil {
		return
	}
	ctx.ShaExistCache = map[string]bool{}
	if ctx.cancelFn == nil {
		return
	}
	ctx.cancelFn()
}

// AddCancel adds the provided fn as a Cleanup for this Ctx
func (ctx *RenderContext) AddCancel(fn func()) {
	if ctx == nil {
		return
	}
	oldCancelFn := ctx.cancelFn
	if oldCancelFn == nil {
		ctx.cancelFn = fn
		return
	}
	ctx.cancelFn = func() {
		defer oldCancelFn()
		fn()
	}
}

func (ctx *RenderContext) IsMarkupContentWiki() bool {
	return ctx.Metas != nil && ctx.Metas["markupContentMode"] == "wiki"
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if ctx.MarkupType == "" && ctx.RelativePath != "" {
		ctx.MarkupType = DetectMarkupTypeByFileName(ctx.RelativePath)
		if ctx.MarkupType == "" {
			return util.NewInvalidArgumentErrorf("unsupported file to render: %q", ctx.RelativePath)
		}
	}

	renderer := renderers[ctx.MarkupType]
	if renderer == nil {
		return util.NewInvalidArgumentErrorf("unsupported markup type: %q", ctx.MarkupType)
	}

	if ctx.RelativePath != "" {
		if externalRender, ok := renderer.(ExternalRenderer); ok && externalRender.DisplayInIFrame() {
			if !ctx.InStandalonePage {
				// for an external "DisplayInIFrame" render, it could only output its content in a standalone page
				// otherwise, a <iframe> should be outputted to embed the external rendered page
				return renderIFrame(ctx, output)
			}
		}
	}

	return render(ctx, renderer, input, output)
}

// RenderString renders Markup string to HTML with all specific handling stuff and return string
func RenderString(ctx *RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderIFrame(ctx *RenderContext, output io.Writer) error {
	// set height="0" ahead, otherwise the scrollHeight would be max(150, realHeight)
	// at the moment, only "allow-scripts" is allowed for sandbox mode.
	// "allow-same-origin" should never be used, it leads to XSS attack, and it makes the JS in iframe can access parent window's config and CSRF token
	// TODO: when using dark theme, if the rendered content doesn't have proper style, the default text color is black, which is not easy to read
	_, err := io.WriteString(output, fmt.Sprintf(`
<iframe src="%s/%s/%s/render/%s/%s"
name="giteaExternalRender"
onload="this.height=giteaExternalRender.document.documentElement.scrollHeight"
width="100%%" height="0" scrolling="no" frameborder="0" style="overflow: hidden"
sandbox="allow-scripts"
></iframe>`,
		setting.AppSubURL,
		url.PathEscape(ctx.Metas["user"]),
		url.PathEscape(ctx.Metas["repo"]),
		ctx.Metas["BranchNameSubURL"],
		url.PathEscape(ctx.RelativePath),
	))
	return err
}

func render(ctx *RenderContext, renderer Renderer, input io.Reader, output io.Writer) error {
	var wg sync.WaitGroup
	var err error
	pr, pw := io.Pipe()
	defer func() {
		_ = pr.Close()
		_ = pw.Close()
	}()

	var pr2 io.ReadCloser
	var pw2 io.WriteCloser

	var sanitizerDisabled bool
	if r, ok := renderer.(ExternalRenderer); ok {
		sanitizerDisabled = r.SanitizerDisabled()
	}

	if !sanitizerDisabled {
		pr2, pw2 = io.Pipe()
		defer func() {
			_ = pr2.Close()
			_ = pw2.Close()
		}()

		wg.Add(1)
		go func() {
			err = SanitizeReader(pr2, renderer.Name(), output)
			_ = pr2.Close()
			wg.Done()
		}()
	} else {
		pw2 = util.NopCloser{Writer: output}
	}

	wg.Add(1)
	go func() {
		if r, ok := renderer.(PostProcessRenderer); ok && r.NeedPostProcess() {
			err = PostProcess(ctx, pr, pw2)
		} else {
			_, err = io.Copy(pw2, pr)
		}
		_ = pr.Close()
		_ = pw2.Close()
		wg.Done()
	}()

	if err1 := renderer.Render(ctx, input, pw); err1 != nil {
		return err1
	}
	_ = pw.Close()

	wg.Wait()
	return err
}

// Init initializes the render global variables
func Init(ph *ProcessorHelper) {
	if ph != nil {
		DefaultProcessorHelper = *ph
	}

	if len(setting.Markdown.CustomURLSchemes) > 0 {
		CustomLinkURLSchemes(setting.Markdown.CustomURLSchemes)
	}

	// since setting maybe changed extensions, this will reload all renderer extensions mapping
	extRenderers = make(map[string]Renderer)
	for _, renderer := range renderers {
		for _, ext := range renderer.Extensions() {
			extRenderers[strings.ToLower(ext)] = renderer
		}
	}
}

func ComposeSimpleDocumentMetas() map[string]string {
	return map[string]string{"markdownLineBreakStyle": "document"}
}
