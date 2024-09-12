// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	texttemplate "text/template"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates/scopedtmpl"
	"code.gitea.io/gitea/modules/util"
)

type TemplateExecutor scopedtmpl.TemplateExecutor

type HTMLRender struct {
	templates atomic.Pointer[scopedtmpl.ScopedTemplate]
}

var (
	htmlRender     *HTMLRender
	htmlRenderOnce sync.Once
)

var ErrTemplateNotInitialized = errors.New("template system is not initialized, check your log for errors")

func (h *HTMLRender) HTML(w io.Writer, status int, name string, data any, ctx context.Context) error { //nolint:revive
	if respWriter, ok := w.(http.ResponseWriter); ok {
		if respWriter.Header().Get("Content-Type") == "" {
			respWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		respWriter.WriteHeader(status)
	}
	t, err := h.TemplateLookup(name, ctx)
	if err != nil {
		return texttemplate.ExecError{Name: name, Err: err}
	}
	return t.Execute(w, data)
}

func (h *HTMLRender) TemplateLookup(name string, ctx context.Context) (TemplateExecutor, error) { //nolint:revive
	tmpls := h.templates.Load()
	if tmpls == nil {
		return nil, ErrTemplateNotInitialized
	}
	m := NewFuncMap()
	m["ctx"] = func() any { return ctx }
	return tmpls.Executor(name, m)
}

func (h *HTMLRender) CompileTemplates() error {
	assets := AssetFS()
	extSuffix := ".tmpl"
	tmpls := scopedtmpl.NewScopedTemplate()
	tmpls.Funcs(NewFuncMap())
	files, err := ListWebTemplateAssetNames(assets)
	if err != nil {
		return nil
	}
	for _, file := range files {
		if !strings.HasSuffix(file, extSuffix) {
			continue
		}
		name := strings.TrimSuffix(file, extSuffix)
		tmpl := tmpls.New(filepath.ToSlash(name))
		buf, err := assets.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err = tmpl.Parse(string(buf)); err != nil {
			return err
		}
	}
	tmpls.Freeze()
	h.templates.Store(tmpls)
	return nil
}

// HTMLRenderer init once and returns the globally shared html renderer
func HTMLRenderer() *HTMLRender {
	htmlRenderOnce.Do(initHTMLRenderer)
	return htmlRender
}

func ReloadHTMLTemplates() error {
	log.Trace("Reloading HTML templates")
	if err := htmlRender.CompileTemplates(); err != nil {
		log.Error("Template error: %v\n%s", err, log.Stack(2))
		return err
	}
	return nil
}

func initHTMLRenderer() {
	rendererType := "static"
	if !setting.IsProd {
		rendererType = "auto-reloading"
	}
	log.Debug("Creating %s HTML Renderer", rendererType)

	htmlRender = &HTMLRender{}
	if err := htmlRender.CompileTemplates(); err != nil {
		p := &templateErrorPrettier{assets: AssetFS()}
		wrapTmplErrMsg(p.handleFuncNotDefinedError(err))
		wrapTmplErrMsg(p.handleUnexpectedOperandError(err))
		wrapTmplErrMsg(p.handleExpectedEndError(err))
		wrapTmplErrMsg(p.handleGenericTemplateError(err))
		wrapTmplErrMsg(fmt.Sprintf("CompileTemplates error: %v", err))
	}

	if !setting.IsProd {
		go AssetFS().WatchLocalChanges(graceful.GetManager().ShutdownContext(), func() {
			_ = ReloadHTMLTemplates()
		})
	}
}

func wrapTmplErrMsg(msg string) {
	if msg == "" {
		return
	}
	if setting.IsProd {
		// in prod mode, Gitea must have correct templates to run
		log.Fatal("Gitea can't run with template errors: %s", msg)
	}
	// in dev mode, do not need to really exit, because the template errors could be fixed by developer soon and the templates get reloaded
	log.Error("There are template errors but Gitea continues to run in dev mode: %s", msg)
}

type templateErrorPrettier struct {
	assets *assetfs.LayeredFS
}

var reGenericTemplateError = regexp.MustCompile(`^template: (.*):([0-9]+): (.*)`)

func (p *templateErrorPrettier) handleGenericTemplateError(err error) string {
	groups := reGenericTemplateError.FindStringSubmatch(err.Error())
	if len(groups) != 4 {
		return ""
	}
	tmplName, lineStr, message := groups[1], groups[2], groups[3]
	return p.makeDetailedError(message, tmplName, lineStr, -1, "")
}

var reFuncNotDefinedError = regexp.MustCompile(`^template: (.*):([0-9]+): (function "(.*)" not defined)`)

func (p *templateErrorPrettier) handleFuncNotDefinedError(err error) string {
	groups := reFuncNotDefinedError.FindStringSubmatch(err.Error())
	if len(groups) != 5 {
		return ""
	}
	tmplName, lineStr, message, funcName := groups[1], groups[2], groups[3], groups[4]
	funcName, _ = strconv.Unquote(`"` + funcName + `"`)
	return p.makeDetailedError(message, tmplName, lineStr, -1, funcName)
}

var reUnexpectedOperandError = regexp.MustCompile(`^template: (.*):([0-9]+): (unexpected "(.*)" in operand)`)

func (p *templateErrorPrettier) handleUnexpectedOperandError(err error) string {
	groups := reUnexpectedOperandError.FindStringSubmatch(err.Error())
	if len(groups) != 5 {
		return ""
	}
	tmplName, lineStr, message, unexpected := groups[1], groups[2], groups[3], groups[4]
	unexpected, _ = strconv.Unquote(`"` + unexpected + `"`)
	return p.makeDetailedError(message, tmplName, lineStr, -1, unexpected)
}

var reExpectedEndError = regexp.MustCompile(`^template: (.*):([0-9]+): (expected end; found (.*))`)

func (p *templateErrorPrettier) handleExpectedEndError(err error) string {
	groups := reExpectedEndError.FindStringSubmatch(err.Error())
	if len(groups) != 5 {
		return ""
	}
	tmplName, lineStr, message, unexpected := groups[1], groups[2], groups[3], groups[4]
	return p.makeDetailedError(message, tmplName, lineStr, -1, unexpected)
}

var (
	reTemplateExecutingError    = regexp.MustCompile(`^template: (.*):([1-9][0-9]*):([1-9][0-9]*): (executing .*)`)
	reTemplateExecutingErrorMsg = regexp.MustCompile(`^executing "(.*)" at <(.*)>: `)
)

func (p *templateErrorPrettier) handleTemplateRenderingError(err error) string {
	if groups := reTemplateExecutingError.FindStringSubmatch(err.Error()); len(groups) > 0 {
		tmplName, lineStr, posStr, msgPart := groups[1], groups[2], groups[3], groups[4]
		target := ""
		if groups = reTemplateExecutingErrorMsg.FindStringSubmatch(msgPart); len(groups) > 0 {
			target = groups[2]
		}
		return p.makeDetailedError(msgPart, tmplName, lineStr, posStr, target)
	} else if execErr, ok := err.(texttemplate.ExecError); ok {
		layerName := p.assets.GetFileLayerName(execErr.Name + ".tmpl")
		return fmt.Sprintf("asset from: %s, %s", layerName, err.Error())
	}
	return err.Error()
}

func HandleTemplateRenderingError(err error) string {
	p := &templateErrorPrettier{assets: AssetFS()}
	return p.handleTemplateRenderingError(err)
}

const dashSeparator = "----------------------------------------------------------------------"

func (p *templateErrorPrettier) makeDetailedError(errMsg, tmplName string, lineNum, posNum any, target string) string {
	code, layer, err := p.assets.ReadLayeredFile(tmplName + ".tmpl")
	if err != nil {
		return fmt.Sprintf("template error: %s, and unable to find template file %q", errMsg, tmplName)
	}
	line, err := util.ToInt64(lineNum)
	if err != nil {
		return fmt.Sprintf("template error: %s, unable to parse template %q line number %q", errMsg, tmplName, lineNum)
	}
	pos, err := util.ToInt64(posNum)
	if err != nil {
		return fmt.Sprintf("template error: %s, unable to parse template %q pos number %q", errMsg, tmplName, posNum)
	}
	detail := extractErrorLine(code, int(line), int(pos), target)

	var msg string
	if pos >= 0 {
		msg = fmt.Sprintf("template error: %s:%s:%d:%d : %s", layer, tmplName, line, pos, errMsg)
	} else {
		msg = fmt.Sprintf("template error: %s:%s:%d : %s", layer, tmplName, line, errMsg)
	}
	return msg + "\n" + dashSeparator + "\n" + detail + "\n" + dashSeparator
}

func extractErrorLine(code []byte, lineNum, posNum int, target string) string {
	b := bufio.NewReader(bytes.NewReader(code))
	var line []byte
	var err error
	for i := 0; i < lineNum; i++ {
		if line, err = b.ReadBytes('\n'); err != nil {
			if i == lineNum-1 && errors.Is(err, io.EOF) {
				err = nil
			}
			break
		}
	}
	if err != nil {
		return fmt.Sprintf("unable to find target line %d", lineNum)
	}

	line = bytes.TrimRight(line, "\r\n")
	var indicatorLine []byte
	targetBytes := []byte(target)
	targetLen := len(targetBytes)
	for i := 0; i < len(line); {
		if posNum == -1 && target != "" && bytes.HasPrefix(line[i:], targetBytes) {
			for j := 0; j < targetLen && i < len(line); j++ {
				indicatorLine = append(indicatorLine, '^')
				i++
			}
		} else if i == posNum {
			indicatorLine = append(indicatorLine, '^')
			i++
		} else {
			if line[i] == '\t' {
				indicatorLine = append(indicatorLine, '\t')
			} else {
				indicatorLine = append(indicatorLine, ' ')
			}
			i++
		}
	}
	// if the indicatorLine only contains spaces, trim it together
	return strings.TrimRight(string(line)+"\n"+string(indicatorLine), " \t\r\n")
}
