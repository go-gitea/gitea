// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/htmlutil"
	"code.gitea.io/gitea/modules/markup/internal"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/yuin/goldmark/ast"
	"golang.org/x/sync/errgroup"
)

type RenderMetaMode string

const (
	RenderMetaAsDetails RenderMetaMode = "details" // default
	RenderMetaAsNone    RenderMetaMode = "none"
	RenderMetaAsTable   RenderMetaMode = "table"
)

var RenderBehaviorForTesting struct {
	// Gitea will emit some additional attributes for various purposes, these attributes don't affect rendering.
	// But there are too many hard-coded test cases, to avoid changing all of them again and again, we can disable emitting these internal attributes.
	DisableAdditionalAttributes bool
}

type RenderOptions struct {
	UseAbsoluteLink bool

	// relative path from tree root of the branch
	RelativePath string

	// eg: "orgmode", "asciicast", "console"
	// for file mode, it could be left as empty, and will be detected by file extension in RelativePath
	MarkupType string

	// user&repo, format&style&regexp (for external issue pattern), teams&org (for mention)
	// RefTypeNameSubURL (for iframe&asciicast)
	// markupAllowShortIssuePattern
	// markdownNewLineHardBreak
	Metas map[string]string

	// used by external render. the router "/org/repo/render/..." will output the rendered content in a standalone page
	InStandalonePage bool
}

// RenderContext represents a render context
type RenderContext struct {
	ctx context.Context

	// the context might be used by the "render" function, but it might also be used by "postProcess" function
	usedByRender bool

	SidebarTocNode ast.Node

	RenderHelper   RenderHelper
	RenderOptions  RenderOptions
	RenderInternal internal.RenderInternal
}

func (ctx *RenderContext) Deadline() (deadline time.Time, ok bool) {
	return ctx.ctx.Deadline()
}

func (ctx *RenderContext) Done() <-chan struct{} {
	return ctx.ctx.Done()
}

func (ctx *RenderContext) Err() error {
	return ctx.ctx.Err()
}

func (ctx *RenderContext) Value(key any) any {
	return ctx.ctx.Value(key)
}

var _ context.Context = (*RenderContext)(nil)

func NewRenderContext(ctx context.Context) *RenderContext {
	return &RenderContext{ctx: ctx, RenderHelper: &SimpleRenderHelper{}}
}

func (ctx *RenderContext) WithMarkupType(typ string) *RenderContext {
	ctx.RenderOptions.MarkupType = typ
	return ctx
}

func (ctx *RenderContext) WithRelativePath(path string) *RenderContext {
	ctx.RenderOptions.RelativePath = path
	return ctx
}

func (ctx *RenderContext) WithMetas(metas map[string]string) *RenderContext {
	ctx.RenderOptions.Metas = metas
	return ctx
}

func (ctx *RenderContext) WithInStandalonePage(v bool) *RenderContext {
	ctx.RenderOptions.InStandalonePage = v
	return ctx
}

func (ctx *RenderContext) WithUseAbsoluteLink(v bool) *RenderContext {
	ctx.RenderOptions.UseAbsoluteLink = v
	return ctx
}

func (ctx *RenderContext) WithHelper(helper RenderHelper) *RenderContext {
	ctx.RenderHelper = helper
	return ctx
}

// FindRendererByContext finds renderer by RenderContext
// TODO: it should be merged with other similar functions like GetRendererByFileName, DetectMarkupTypeByFileName, etc
func FindRendererByContext(ctx *RenderContext) (Renderer, error) {
	if ctx.RenderOptions.MarkupType == "" && ctx.RenderOptions.RelativePath != "" {
		ctx.RenderOptions.MarkupType = DetectMarkupTypeByFileName(ctx.RenderOptions.RelativePath)
		if ctx.RenderOptions.MarkupType == "" {
			return nil, util.NewInvalidArgumentErrorf("unsupported file to render: %q", ctx.RenderOptions.RelativePath)
		}
	}

	renderer := renderers[ctx.RenderOptions.MarkupType]
	if renderer == nil {
		return nil, util.NewNotExistErrorf("unsupported markup type: %q", ctx.RenderOptions.MarkupType)
	}

	return renderer, nil
}

func RendererNeedPostProcess(renderer Renderer) bool {
	if r, ok := renderer.(PostProcessRenderer); ok && r.NeedPostProcess() {
		return true
	}
	return false
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	renderer, err := FindRendererByContext(ctx)
	if err != nil {
		return err
	}
	return RenderWithRenderer(ctx, renderer, input, output)
}

// RenderString renders Markup string to HTML with all specific handling stuff and return string
func RenderString(ctx *RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderIFrame(ctx *RenderContext, sandbox string, output io.Writer) error {
	src := fmt.Sprintf("%s/%s/%s/render/%s/%s", setting.AppSubURL,
		url.PathEscape(ctx.RenderOptions.Metas["user"]),
		url.PathEscape(ctx.RenderOptions.Metas["repo"]),
		util.PathEscapeSegments(ctx.RenderOptions.Metas["RefTypeNameSubURL"]),
		util.PathEscapeSegments(ctx.RenderOptions.RelativePath),
	)

	var sandboxAttrValue template.HTML
	if sandbox != "" {
		sandboxAttrValue = htmlutil.HTMLFormat(`sandbox="%s"`, sandbox)
	}
	iframe := htmlutil.HTMLFormat(`<iframe data-src="%s" class="external-render-iframe" %s></iframe>`, src, sandboxAttrValue)
	_, err := io.WriteString(output, string(iframe))
	return err
}

func pipes() (io.ReadCloser, io.WriteCloser, func()) {
	pr, pw := io.Pipe()
	return pr, pw, func() {
		_ = pr.Close()
		_ = pw.Close()
	}
}

func getExternalRendererOptions(renderer Renderer) (ret ExternalRendererOptions, _ bool) {
	if externalRender, ok := renderer.(ExternalRenderer); ok {
		return externalRender.GetExternalRendererOptions(), true
	}
	return ret, false
}

func RenderWithRenderer(ctx *RenderContext, renderer Renderer, input io.Reader, output io.Writer) error {
	var extraHeadHTML template.HTML
	if extOpts, ok := getExternalRendererOptions(renderer); ok && extOpts.DisplayInIframe {
		if !ctx.RenderOptions.InStandalonePage {
			// for an external "DisplayInIFrame" render, it could only output its content in a standalone page
			// otherwise, a <iframe> should be outputted to embed the external rendered page
			return renderIFrame(ctx, extOpts.ContentSandbox, output)
		}
		// else: this is a standalone page, fallthrough to the real rendering, and add extra JS/CSS
		extraStyleHref := setting.AppSubURL + "/assets/css/external-render-iframe.css"
		extraScriptSrc := setting.AppSubURL + "/assets/js/external-render-iframe.js"
		// "<script>" must go before "<link>", to make Golang's http.DetectContentType() can still recognize the content as "text/html"
		extraHeadHTML = htmlutil.HTMLFormat(`<script src="%s"></script><link rel="stylesheet" href="%s">`, extraScriptSrc, extraStyleHref)
	}

	ctx.usedByRender = true
	if ctx.RenderHelper != nil {
		defer ctx.RenderHelper.CleanUp()
	}

	finalProcessor := ctx.RenderInternal.Init(output, extraHeadHTML)
	defer finalProcessor.Close()

	// input -> (pw1=pr1) -> renderer -> (pw2=pr2) -> SanitizeReader -> finalProcessor -> output
	// no sanitizer: input -> (pw1=pr1) -> renderer -> pw2(finalProcessor) -> output
	pr1, pw1, close1 := pipes()
	defer close1()

	eg, _ := errgroup.WithContext(ctx)
	var pw2 io.WriteCloser = util.NopCloser{Writer: finalProcessor}

	if r, ok := renderer.(ExternalRenderer); !ok || !r.GetExternalRendererOptions().SanitizerDisabled {
		var pr2 io.ReadCloser
		var close2 func()
		pr2, pw2, close2 = pipes()
		defer close2()
		eg.Go(func() error {
			defer pr2.Close()
			return SanitizeReader(pr2, renderer.Name(), finalProcessor)
		})
	}

	eg.Go(func() (err error) {
		if RendererNeedPostProcess(renderer) {
			err = PostProcessDefault(ctx, pr1, pw2)
		} else {
			_, err = io.Copy(pw2, pr1)
		}
		_, _ = pr1.Close(), pw2.Close()
		return err
	})

	if err := renderer.Render(ctx, input, pw1); err != nil {
		return err
	}
	_ = pw1.Close()

	return eg.Wait()
}

// Init initializes the render global variables
func Init(renderHelpFuncs *RenderHelperFuncs) {
	DefaultRenderHelperFuncs = renderHelpFuncs
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
	// TODO: there is no separate config option for "simple document" rendering, so temporarily use the same config as "repo file"
	return map[string]string{"markdownNewLineHardBreak": strconv.FormatBool(setting.Markdown.RenderOptionsRepoFile.NewLineHardBreak)}
}

type TestRenderHelper struct {
	ctx      *RenderContext
	BaseLink string
}

func (r *TestRenderHelper) CleanUp() {}

func (r *TestRenderHelper) IsCommitIDExisting(commitID string) bool {
	return strings.HasPrefix(commitID, "65f1bf2") //|| strings.HasPrefix(commitID, "88fc37a")
}

func (r *TestRenderHelper) ResolveLink(link, preferLinkType string) string {
	linkType, link := ParseRenderedLink(link, preferLinkType)
	switch linkType {
	case LinkTypeRoot:
		return r.ctx.ResolveLinkRoot(link)
	default:
		return r.ctx.ResolveLinkRelative(r.BaseLink, "", link)
	}
}

var _ RenderHelper = (*TestRenderHelper)(nil)

// NewTestRenderContext is a helper function to create a RenderContext for testing purpose
// It accepts string (BaseLink), map[string]string (Metas)
func NewTestRenderContext(baseLinkOrMetas ...any) *RenderContext {
	if !setting.IsInTesting {
		panic("NewTestRenderContext should only be used in testing")
	}
	helper := &TestRenderHelper{}
	ctx := NewRenderContext(context.Background()).WithHelper(helper)
	helper.ctx = ctx
	for _, v := range baseLinkOrMetas {
		switch v := v.(type) {
		case string:
			helper.BaseLink = v
		case map[string]string:
			ctx = ctx.WithMetas(v)
		default:
			panic(fmt.Sprintf("unknown type %T", v))
		}
	}
	return ctx
}
