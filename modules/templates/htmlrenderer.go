// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/watcher"

	"github.com/unrolled/render"
)

var rendererKey interface{} = "templatesHtmlRendereer"

// HTMLRenderer returns the current html renderer for the context or creates and stores one within the context for future use
func HTMLRenderer(ctx context.Context) (context.Context, *render.Render) {
	rendererInterface := ctx.Value(rendererKey)
	if rendererInterface != nil {
		renderer, ok := rendererInterface.(*render.Render)
		if ok {
			return ctx, renderer
		}
	}

	rendererType := "static"
	if !setting.IsProd {
		rendererType = "auto-reloading"
	}
	log.Log(1, log.DEBUG, "Creating "+rendererType+" HTML Renderer")

	renderer := render.New(render.Options{
		Extensions:                []string{".tmpl"},
		Directory:                 "templates",
		Funcs:                     NewFuncMap(),
		Asset:                     GetAsset,
		AssetNames:                GetTemplateAssetNames,
		UseMutexLock:              !setting.IsProd,
		IsDevelopment:             false,
		DisableHTTPErrorRendering: true,
	})
	if !setting.IsProd {
		watcher.CreateWatcher(ctx, "HTML Templates", &watcher.CreateWatcherOpts{
			PathsCallback:   walkTemplateFiles,
			BetweenCallback: renderer.CompileTemplates,
		})
	}
	return context.WithValue(ctx, rendererKey, renderer), renderer
}
