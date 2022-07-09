// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/services/auth"

	"gitea.com/go-chi/session"
)

func storageHandler(storageSetting setting.Storage, prefix string, objStore storage.ObjectStorage) func(next http.Handler) http.Handler {
	prefix = strings.Trim(prefix, "/")
	funcInfo := routing.GetFuncInfo(storageHandler, prefix)
	return func(next http.Handler) http.Handler {
		if storageSetting.ServeDirect {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" && req.Method != "HEAD" {
					next.ServeHTTP(w, req)
					return
				}

				if !strings.HasPrefix(req.URL.Path, "/"+prefix+"/") {
					next.ServeHTTP(w, req)
					return
				}
				routing.UpdateFuncInfo(req.Context(), funcInfo)

				rPath := strings.TrimPrefix(req.URL.Path, "/"+prefix+"/")
				rPath = path.Clean("/" + strings.ReplaceAll(rPath, "\\", "/"))[1:]

				u, err := objStore.URL(rPath, path.Base(rPath))
				if err != nil {
					if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
						log.Warn("Unable to find %s %s", prefix, rPath)
						http.Error(w, "file not found", http.StatusNotFound)
						return
					}
					log.Error("Error whilst getting URL for %s %s. Error: %v", prefix, rPath, err)
					http.Error(w, fmt.Sprintf("Error whilst getting URL for %s %s", prefix, rPath), http.StatusInternalServerError)
					return
				}

				http.Redirect(
					w,
					req,
					u.String(),
					http.StatusTemporaryRedirect,
				)
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != "GET" && req.Method != "HEAD" {
				next.ServeHTTP(w, req)
				return
			}

			if !strings.HasPrefix(req.URL.Path, "/"+prefix+"/") {
				next.ServeHTTP(w, req)
				return
			}
			routing.UpdateFuncInfo(req.Context(), funcInfo)

			rPath := strings.TrimPrefix(req.URL.Path, "/"+prefix+"/")
			rPath = path.Clean("/" + strings.ReplaceAll(rPath, "\\", "/"))[1:]
			if rPath == "" {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			}

			fi, err := objStore.Stat(rPath)
			if err == nil && httpcache.HandleTimeCache(req, w, fi) {
				return
			}

			// If we have matched and access to release or issue
			fr, err := objStore.Open(rPath)
			if err != nil {
				if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
					log.Warn("Unable to find %s %s", prefix, rPath)
					http.Error(w, "file not found", http.StatusNotFound)
					return
				}
				log.Error("Error whilst opening %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst opening %s %s", prefix, rPath), http.StatusInternalServerError)
				return
			}
			defer fr.Close()

			_, err = io.Copy(w, fr)
			if err != nil {
				log.Error("Error whilst rendering %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst rendering %s %s", prefix, rPath), http.StatusInternalServerError)
				return
			}
		})
	}
}

type dataStore map[string]interface{}

func (d *dataStore) GetData() map[string]interface{} {
	return *d
}

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// This error will be created with the gitea 500 page.
func Recovery() func(next http.Handler) http.Handler {
	rnd := templates.HTMLRenderer()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					routing.UpdatePanicError(req.Context(), err)
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, log.Stack(2))
					log.Error("%s", combinedErr)

					sessionStore := session.GetSession(req)

					lc := middleware.Locale(w, req)
					store := dataStore{
						"Language":   lc.Language(),
						"CurrentURL": setting.AppSubURL + req.URL.RequestURI(),
						"locale":     lc,
					}

					user := context.GetContextUser(req)
					if user == nil {
						// Get user from session if logged in - do not attempt to sign-in
						user = auth.SessionUser(sessionStore)
					}
					if user != nil {
						store["IsSigned"] = true
						store["SignedUser"] = user
						store["SignedUserID"] = user.ID
						store["SignedUserName"] = user.Name
						store["IsAdmin"] = user.IsAdmin
					} else {
						store["SignedUserID"] = int64(0)
						store["SignedUserName"] = ""
					}

					w.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

					if !setting.IsProd {
						store["ErrorMsg"] = combinedErr
					}
					err = rnd.HTML(w, http.StatusInternalServerError, "status/500", templates.BaseVars().Merge(store))
					if err != nil {
						log.Error("%v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
