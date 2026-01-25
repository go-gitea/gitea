// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package templates

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	texttemplate "text/template"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

type pageRenderer struct {
	tmplRenderer *tmplRender
}

func (r *pageRenderer) funcMap(ctx context.Context) template.FuncMap {
	pageFuncMap := NewFuncMap()
	pageFuncMap["ctx"] = func() any { return ctx }
	return pageFuncMap
}

func (r *pageRenderer) funcMapDummy() template.FuncMap {
	dummyFuncMap := NewFuncMap()
	dummyFuncMap["ctx"] = func() any { return nil } // for template compilation only, no context available
	return dummyFuncMap
}

func (r *pageRenderer) TemplateLookup(tmpl string, templateCtx context.Context) (TemplateExecutor, error) { //nolint:revive // we don't use ctx, only pass it to the template executor
	return r.tmplRenderer.Templates().Executor(tmpl, r.funcMap(templateCtx))
}

func (r *pageRenderer) HTML(w io.Writer, status int, tplName TplName, data any, templateCtx context.Context) error { //nolint:revive // we don't use ctx, only pass it to the template executor
	name := string(tplName)
	if respWriter, ok := w.(http.ResponseWriter); ok {
		if respWriter.Header().Get("Content-Type") == "" {
			respWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		respWriter.WriteHeader(status)
	}
	t, err := r.TemplateLookup(name, templateCtx)
	if err != nil {
		return texttemplate.ExecError{Name: name, Err: err}
	}
	return t.Execute(w, data)
}

var PageRenderer = sync.OnceValue(func() *pageRenderer {
	rendererType := util.Iif(setting.IsProd, "static", "auto-reloading")
	log.Debug("Creating %s HTML Renderer", rendererType)

	assetFS := AssetFS()
	tr := &tmplRender{
		collectTemplateNames: func() ([]string, error) {
			names, err := assetFS.ListAllFiles(".", true)
			if err != nil {
				return nil, err
			}
			names = slices.DeleteFunc(names, func(file string) bool {
				return strings.HasPrefix(file, "mail/") || !strings.HasSuffix(file, ".tmpl")
			})
			for i, file := range names {
				names[i] = strings.TrimSuffix(file, ".tmpl")
			}
			return names, nil
		},
		readTemplateContent: func(name string) ([]byte, error) {
			return assetFS.ReadFile(name + ".tmpl")
		},
	}

	pr := &pageRenderer{tmplRenderer: tr}
	if err := tr.recompileTemplates(pr.funcMapDummy()); err != nil {
		processStartupTemplateError(err)
	}

	if !setting.IsProd {
		go AssetFS().WatchLocalChanges(graceful.GetManager().ShutdownContext(), func() {
			if err := tr.recompileTemplates(pr.funcMapDummy()); err != nil {
				log.Error("Template error: %v\n%s", err, log.Stack(2))
			}
		})
	}
	return pr
})

func PageRendererReload() error {
	return PageRenderer().tmplRenderer.recompileTemplates(PageRenderer().funcMapDummy())
}
