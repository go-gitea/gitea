// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	goctx "context"
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
	"code.gitea.io/gitea/modules/util"
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
				rPath = util.PathJoinRelX(rPath)

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
			rPath = util.PathJoinRelX(rPath)
			if rPath == "" || rPath == "." {
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
func Recovery(ctx goctx.Context) func(next http.Handler) http.Handler {
	_, rnd := templates.HTMLRenderer(ctx)
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

					// TODO: this recovery handler is usually called without Gitea's web context, so we shouldn't touch that context too much
					// Otherwise, the 500 page may cause new panics, eg: cache.GetContextWithData, it makes the developer&users couldn't find the original panic
					user := context.GetContextUser(req) // almost always nil
					if user == nil {
						// Get user from session if logged in - do not attempt to sign-in
						user = auth.SessionUser(sessionStore)
					}

					httpcache.SetCacheControlInHeader(w.Header(), 0, "no-transform")
					w.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

					if !setting.IsProd || (user != nil && user.IsAdmin) {
						store["ErrorMsg"] = combinedErr
					}

					defer func() {
						if err := recover(); err != nil {
							log.Error("HTML render in Recovery handler panics again: %v", err)
						}
					}()
					err = rnd.HTML(w, http.StatusInternalServerError, "status/500", templates.BaseVars().Merge(store))
					if err != nil {
						log.Error("HTML render in Recovery handler fails again: %v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
