// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/auth/sso"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/middlewares"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"

	"github.com/unrolled/render"
)

type dataStore struct {
	Data map[string]interface{}
}

func (d *dataStore) GetData() map[string]interface{} {
	return d.Data
}

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
			IsDevelopment: !setting.IsProd(),
		})

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				// Why we need this? The first recover will try to render a beautiful
				// error page for user, but the process can still panic again, then
				// we have to just recover twice and send a simple error page that
				// should not panic any more.
				defer func() {
					if err := recover(); err != nil {
						combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, string(log.Stack(2)))
						log.Error(combinedErr)
						if setting.IsProd() {
							http.Error(w, http.StatusText(500), 500)
						} else {
							http.Error(w, combinedErr, 500)
						}
					}
				}()

				if err := recover(); err != nil {
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, string(log.Stack(2)))
					log.Error("%v", combinedErr)

					lc := middlewares.Locale(w, req)

					// TODO: this should be replaced by real session after macaron removed totally
					sessionStore, err := sessionManager.Start(w, req)
					if err != nil {
						// Just invoke the above recover catch
						panic("session(start): " + err.Error())
					}

					var store = dataStore{
						Data: templates.Vars{
							"Language":   lc.Language(),
							"CurrentURL": setting.AppSubURL + req.URL.RequestURI(),
							"i18n":       lc,
						},
					}

					// Get user from session if logged in.
					user, _ := sso.SignedInUser(req, w, &store, sessionStore)
					if user != nil {
						store.Data["IsSigned"] = true
						store.Data["SignedUser"] = user
						store.Data["SignedUserID"] = user.ID
						store.Data["SignedUserName"] = user.Name
						store.Data["IsAdmin"] = user.IsAdmin
					} else {
						store.Data["SignedUserID"] = int64(0)
						store.Data["SignedUserName"] = ""
					}

					w.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

					if !setting.IsProd() {
						store.Data["ErrorMsg"] = combinedErr
					}
					err = rnd.HTML(w, 500, "status/500", templates.BaseVars().Merge(store.Data))
					if err != nil {
						log.Error("%v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
