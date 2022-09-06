// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

// Init initialize regexps for markdown parsing
func Init() {
	NewSanitizer()
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

// Header holds the data about a header.
type Header struct {
	Level int
	Text  string
	ID    string
}

// RenderContext represents a render context
type RenderContext struct {
	Ctx              context.Context
	RelativePath     string // relative path from tree root of the branch
	Type             string
	IsWiki           bool
	URLPrefix        string
	Metas            map[string]string
	DefaultLink      string
	GitRepo          *git.Repository
	ShaExistCache    map[string]bool
	cancelFn         func()
	TableOfContents  []Header
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

// Renderer defines an interface for rendering markup file to HTML
type Renderer interface {
	Name() string // markup format name
	Extensions() []string
	SanitizerRules() []setting.MarkupSanitizerRule
	Render(ctx *RenderContext, input io.Reader, output io.Writer) error
}

// PostProcessRenderer defines an interface for renderers who need post process
type PostProcessRenderer interface {
	NeedPostProcess() bool
}

// PostProcessRenderer defines an interface for external renderers
type ExternalRenderer interface {
	// SanitizerDisabled disabled sanitize if return true
	SanitizerDisabled() bool

	// DisplayInIFrame represents whether render the content with an iframe
	DisplayInIFrame() bool

	// DisplayAsPDF represents whether to the renderer output should be viewed as PDF.
	DisplayAsPDF() bool
}

// RendererContentDetector detects if the content can be rendered
// by specified renderer
type RendererContentDetector interface {
	CanRender(filename string, input io.Reader) bool
}

var (
	extRenderers = make(map[string]Renderer)
	renderers    = make(map[string]Renderer)
)

// RegisterRenderer registers a new markup file renderer
func RegisterRenderer(renderer Renderer) {
	renderers[renderer.Name()] = renderer
	for _, ext := range renderer.Extensions() {
		extRenderers[strings.ToLower(ext)] = renderer
	}
}

// GetRendererByFileName get renderer by filename
func GetRendererByFileName(filename string) Renderer {
	extension := strings.ToLower(filepath.Ext(filename))
	return extRenderers[extension]
}

// GetRendererByType returns a renderer according type
func GetRendererByType(tp string) Renderer {
	return renderers[tp]
}

// DetectRendererType detects the markup type of the content
func DetectRendererType(filename string, input io.Reader) string {
	buf, err := io.ReadAll(input)
	if err != nil {
		return ""
	}
	for _, renderer := range renderers {
		if detector, ok := renderer.(RendererContentDetector); ok && detector.CanRender(filename, bytes.NewReader(buf)) {
			return renderer.Name()
		}
	}
	return ""
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type != "" {
		return renderByType(ctx, input, output)
	} else if ctx.RelativePath != "" {
		return renderFile(ctx, input, output)
	}
	return errors.New("Render options both filename and type missing")
}

// RenderString renders Markup string to HTML with all specific handling stuff and return string
func RenderString(ctx *RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

func getRenderURL(ctx *RenderContext) string {
	return fmt.Sprintf("%s/%s/%s/render/%s/%s",
		setting.AppSubURL,
		url.PathEscape(ctx.Metas["user"]),
		url.PathEscape(ctx.Metas["repo"]),
		ctx.Metas["BranchNameSubURL"],
		url.PathEscape(ctx.RelativePath),
	)
}

func renderIFrame(ctx *RenderContext, output io.Writer) error {
	// set height="0" ahead, otherwise the scrollHeight would be max(150, realHeight)
	// at the moment, only "allow-scripts" is allowed for sandbox mode.
	// "allow-same-origin" should never be used, it leads to XSS attack, and it makes the JS in iframe can access parent window's config and CSRF token
	// TODO: when using dark theme, if the rendered content doesn't have proper style, the default text color is black, which is not easy to read
	_, err := io.WriteString(output, fmt.Sprintf(`
<iframe src="%s"
name="giteaExternalRender"
onload="this.height=giteaExternalRender.document.documentElement.scrollHeight"
width="100%%" height="0" scrolling="no" frameborder="0" style="overflow: hidden"
sandbox="allow-scripts"
></iframe>`,
		getRenderURL(ctx),
	))
	return err
}

func renderPDFViewer(ctx *RenderContext, output io.Writer) error {
	_, err := io.WriteString(output, fmt.Sprintf(`
<iframe width="100%%" height="600px" src="%s/vendor/plugins/pdfjs/web/viewer.html?file=%s"></iframe>`,
		setting.StaticURLPrefix+"/assets",
		getRenderURL(ctx),
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
		pw2 = nopCloser{output}
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

// ErrUnsupportedRenderType represents
type ErrUnsupportedRenderType struct {
	Type string
}

func (err ErrUnsupportedRenderType) Error() string {
	return fmt.Sprintf("Unsupported render type: %s", err.Type)
}

func renderByType(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if renderer, ok := renderers[ctx.Type]; ok {
		return render(ctx, renderer, input, output)
	}
	return ErrUnsupportedRenderType{ctx.Type}
}

// ErrUnsupportedRenderExtension represents the error when extension doesn't supported to render
type ErrUnsupportedRenderExtension struct {
	Extension string
}

func (err ErrUnsupportedRenderExtension) Error() string {
	return fmt.Sprintf("Unsupported render extension: %s", err.Extension)
}

func renderFile(ctx *RenderContext, input io.Reader, output io.Writer) error {
	extension := strings.ToLower(filepath.Ext(ctx.RelativePath))
	if renderer, ok := extRenderers[extension]; ok {
		if r, ok := renderer.(ExternalRenderer); ok && !ctx.InStandalonePage {
			// for an external render, it could only output its content in a standalone page
			// otherwise, the output may need to be embedded into an <iframe> or a viewer
			if r.DisplayInIFrame() {
				return renderIFrame(ctx, output)
			} else if r.DisplayAsPDF() {
				return renderPDFViewer(ctx, output)
			}
		}
		return render(ctx, renderer, input, output)
	}
	return ErrUnsupportedRenderExtension{extension}
}

// Type returns if markup format via the filename
func Type(filename string) string {
	if parser := GetRendererByFileName(filename); parser != nil {
		return parser.Name()
	}
	return ""
}

// IsMarkupFile reports whether file is a markup type file
func IsMarkupFile(name, markup string) bool {
	if parser := GetRendererByFileName(name); parser != nil {
		return parser.Name() == markup
	}
	return false
}

// IsReadmeFile reports whether name looks like a README file
// based on its name.
func IsReadmeFile(name string) bool {
	name = strings.ToLower(name)
	if len(name) < 6 {
		return false
	} else if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}

// IsReadmeFileExtension reports whether name looks like a README file
// based on its name. It will look through the provided extensions and check if the file matches
// one of the extensions and provide the index in the extension list.
// If the filename is `readme.` with an unmatched extension it will match with the index equaling
// the length of the provided extension list.
// Note that the '.' should be provided in ext, e.g ".md"
func IsReadmeFileExtension(name string, ext ...string) (int, bool) {
	name = strings.ToLower(name)
	if len(name) < 6 || name[:6] != "readme" {
		return 0, false
	}

	for i, extension := range ext {
		extension = strings.ToLower(extension)
		if name[6:] == extension {
			return i, true
		}
	}

	if name[6] == '.' {
		return len(ext), true
	}

	return 0, false
}
