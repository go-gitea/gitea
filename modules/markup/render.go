// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
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

type RenderOptions struct {
	// relative path from tree root of the branch
	RelativePath string

	// eg: "orgmode", "asciicast", "console"
	// for file mode, it could be left as empty, and will be detected by file extension in RelativePath
	MarkupType string

	// special link references for rendering, especially when there is a branch/tree path
	Links Links

	// user&repo, format&style&regexp (for external issue pattern), teams&org (for mention)
	// BranchNameSubURL (for iframe&asciicast)
	// markupAllowShortIssuePattern, markupContentMode (wiki)
	// markdownLineBreakStyle (comment, document)
	Metas map[string]string

	// used by external render. the router "/org/repo/render/..." will output the rendered content in a standalone page
	InStandalonePage bool
}

type RenderHelper struct {
	gitRepo       *git.Repository
	repoFacade    gitrepo.Repository
	shaExistCache map[string]bool
	cancelFn      func()
}

// RenderContext represents a render context
type RenderContext struct {
	ctx context.Context

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
	return &RenderContext{ctx: ctx}
}

func (ctx *RenderContext) WithMarkupType(typ string) *RenderContext {
	ctx.RenderOptions.MarkupType = typ
	return ctx
}

func (ctx *RenderContext) WithRelativePath(path string) *RenderContext {
	ctx.RenderOptions.RelativePath = path
	return ctx
}

func (ctx *RenderContext) WithLinks(links Links) *RenderContext {
	ctx.RenderOptions.Links = links
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

func (ctx *RenderContext) WithGitRepo(r *git.Repository) *RenderContext {
	ctx.RenderHelper.gitRepo = r
	return ctx
}

func (ctx *RenderContext) WithRepoFacade(r gitrepo.Repository) *RenderContext {
	ctx.RenderHelper.repoFacade = r
	return ctx
}

// Cancel runs any cleanup functions that have been registered for this Ctx
func (ctx *RenderContext) Cancel() {
	if ctx == nil {
		return
	}
	ctx.RenderHelper.shaExistCache = map[string]bool{}
	if ctx.RenderHelper.cancelFn == nil {
		return
	}
	ctx.RenderHelper.cancelFn()
}

// AddCancel adds the provided fn as a Cleanup for this Ctx
func (ctx *RenderContext) AddCancel(fn func()) {
	if ctx == nil {
		return
	}
	oldCancelFn := ctx.RenderHelper.cancelFn
	if oldCancelFn == nil {
		ctx.RenderHelper.cancelFn = fn
		return
	}
	ctx.RenderHelper.cancelFn = func() {
		defer oldCancelFn()
		fn()
	}
}

func (ctx *RenderContext) IsMarkupContentWiki() bool {
	return ctx.RenderOptions.Metas != nil && ctx.RenderOptions.Metas["markupContentMode"] == "wiki"
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if ctx.RenderOptions.MarkupType == "" && ctx.RenderOptions.RelativePath != "" {
		ctx.RenderOptions.MarkupType = DetectMarkupTypeByFileName(ctx.RenderOptions.RelativePath)
		if ctx.RenderOptions.MarkupType == "" {
			return util.NewInvalidArgumentErrorf("unsupported file to render: %q", ctx.RenderOptions.RelativePath)
		}
	}

	renderer := renderers[ctx.RenderOptions.MarkupType]
	if renderer == nil {
		return util.NewInvalidArgumentErrorf("unsupported markup type: %q", ctx.RenderOptions.MarkupType)
	}

	if ctx.RenderOptions.RelativePath != "" {
		if externalRender, ok := renderer.(ExternalRenderer); ok && externalRender.DisplayInIFrame() {
			if !ctx.RenderOptions.InStandalonePage {
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
		url.PathEscape(ctx.RenderOptions.Metas["user"]),
		url.PathEscape(ctx.RenderOptions.Metas["repo"]),
		ctx.RenderOptions.Metas["BranchNameSubURL"],
		url.PathEscape(ctx.RenderOptions.RelativePath),
	))
	return err
}

func pipes() (io.ReadCloser, io.WriteCloser, func()) {
	pr, pw := io.Pipe()
	return pr, pw, func() {
		_ = pr.Close()
		_ = pw.Close()
	}
}

func render(ctx *RenderContext, renderer Renderer, input io.Reader, output io.Writer) error {
	finalProcessor := ctx.RenderInternal.Init(output)
	defer finalProcessor.Close()

	// input -> (pw1=pr1) -> renderer -> (pw2=pr2) -> SanitizeReader -> finalProcessor -> output
	// no sanitizer: input -> (pw1=pr1) -> renderer -> pw2(finalProcessor) -> output
	pr1, pw1, close1 := pipes()
	defer close1()

	eg, _ := errgroup.WithContext(ctx)
	var pw2 io.WriteCloser = util.NopCloser{Writer: finalProcessor}

	if r, ok := renderer.(ExternalRenderer); !ok || !r.SanitizerDisabled() {
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
		if r, ok := renderer.(PostProcessRenderer); ok && r.NeedPostProcess() {
			err = PostProcess(ctx, pr1, pw2)
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

// NewTestRenderContext is a helper function to create a RenderContext for testing purpose
// It accepts string (RelativePath), Links, map[string]string (Metas), gitrepo.Repository
func NewTestRenderContext(a ...any) *RenderContext {
	if !setting.IsInTesting {
		panic("NewTestRenderContext should only be used in testing")
	}
	ctx := NewRenderContext(context.Background())
	for _, v := range a {
		switch v := v.(type) {
		case string:
			ctx = ctx.WithRelativePath(v)
		case Links:
			ctx = ctx.WithLinks(v)
		case map[string]string:
			ctx = ctx.WithMetas(v)
		case gitrepo.Repository:
			ctx = ctx.WithRepoFacade(v)
		default:
			panic(fmt.Sprintf("unknown type %T", v))
		}
	}
	return ctx
}
