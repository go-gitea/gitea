// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middlewares

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"

	"github.com/unrolled/render"
)

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// Although similar to macaron.Recovery() the main difference is that this error will be created
// with the gitea 500 page.
func Recovery() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		rnd := render.New(render.Options{
			Extensions:    []string{".tmpl"},
			Directory:     "templates",
			Funcs:         templates.NewFuncMap(),
			Asset:         templates.GetAsset,
			AssetNames:    templates.GetAssetNames,
			IsDevelopment: setting.RunMode != "prod",
		})

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, string(log.Stack(2)))
					log.Error("%v", combinedErr)

					lc := Locale(w, req)

					var (
						data = templates.Vars{
							"Language":   lc.Language(),
							"CurrentURL": setting.AppSubURL + req.URL.RequestURI(),
							"i18n":       lc,
						}
					)
					if setting.RunMode != "prod" {
						data["ErrMsg"] = combinedErr
					}
					err := rnd.HTML(w, 500, "status/500", templates.BaseVars().Merge(data))
					if err != nil {
						log.Error("%v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
