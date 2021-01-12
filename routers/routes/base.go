// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"code.gitea.io/gitea/modules/auth/sso"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/middlewares"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web"

	"gitea.com/go-chi/session"
	"github.com/go-chi/chi/middleware"
)

type routerLoggerOptions struct {
	req            *http.Request
	Identity       *string
	Start          *time.Time
	ResponseWriter http.ResponseWriter
}

// SignedUserName returns signed user's name via context
// FIXME currently no any data stored on chi.Context but macaron.Context, so this will
// return "" before we remove macaron totally
func SignedUserName(req *http.Request) string {
	if v, ok := req.Context().Value("SignedUserName").(string); ok {
		return v
	}
	return ""
}

func accessLogger() func(http.Handler) http.Handler {
	logger := log.GetLogger("access")
	logTemplate, _ := template.New("log").Parse(setting.AccessLogTemplate)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, req)
			identity := "-"
			if val := SignedUserName(req); val != "" {
				identity = val
			}
			rw := w

			buf := bytes.NewBuffer([]byte{})
			err := logTemplate.Execute(buf, routerLoggerOptions{
				req:            req,
				Identity:       &identity,
				Start:          &start,
				ResponseWriter: rw,
			})
			if err != nil {
				log.Error("Could not set up macaron access logger: %v", err.Error())
			}

			err = logger.SendLog(log.INFO, "", "", 0, buf.String(), "")
			if err != nil {
				log.Error("Could not set up macaron access logger: %v", err.Error())
			}
		})
	}
}

// LoggerHandler is a handler that will log the routing to the default gitea log
func LoggerHandler(level log.Level) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			_ = log.GetLogger("router").Log(0, level, "Started %s %s for %s", log.ColoredMethod(req.Method), req.URL.RequestURI(), req.RemoteAddr)

			next.ServeHTTP(w, req)

			var status int
			if v, ok := w.(context.ResponseWriter); ok {
				status = v.Status()
			}

			_ = log.GetLogger("router").Log(0, level, "Completed %s %s %v %s in %v", log.ColoredMethod(req.Method), req.URL.RequestURI(), log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(start)))
		})
	}
}

func storageHandler(storageSetting setting.Storage, prefix string, objStore storage.ObjectStorage) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if storageSetting.ServeDirect {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" && req.Method != "HEAD" {
					next.ServeHTTP(w, req)
					return
				}

				if !strings.HasPrefix(req.URL.RequestURI(), "/"+prefix) {
					next.ServeHTTP(w, req)
					return
				}

				rPath := strings.TrimPrefix(req.URL.RequestURI(), "/"+prefix)
				u, err := objStore.URL(rPath, path.Base(rPath))
				if err != nil {
					if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
						log.Warn("Unable to find %s %s", prefix, rPath)
						http.Error(w, "file not found", 404)
						return
					}
					log.Error("Error whilst getting URL for %s %s. Error: %v", prefix, rPath, err)
					http.Error(w, fmt.Sprintf("Error whilst getting URL for %s %s", prefix, rPath), 500)
					return
				}
				http.Redirect(
					w,
					req,
					u.String(),
					301,
				)
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != "GET" && req.Method != "HEAD" {
				next.ServeHTTP(w, req)
				return
			}

			if !strings.HasPrefix(req.URL.RequestURI(), "/"+prefix) {
				next.ServeHTTP(w, req)
				return
			}

			rPath := strings.TrimPrefix(req.URL.RequestURI(), "/"+prefix)
			rPath = strings.TrimPrefix(rPath, "/")

			fi, err := objStore.Stat(rPath)
			if err == nil && httpcache.HandleTimeCache(req, w, fi) {
				return
			}

			//If we have matched and access to release or issue
			fr, err := objStore.Open(rPath)
			if err != nil {
				if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
					log.Warn("Unable to find %s %s", prefix, rPath)
					http.Error(w, "file not found", 404)
					return
				}
				log.Error("Error whilst opening %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst opening %s %s", prefix, rPath), 500)
				return
			}
			defer fr.Close()

			_, err = io.Copy(w, fr)
			if err != nil {
				log.Error("Error whilst rendering %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst rendering %s %s", prefix, rPath), 500)
				return
			}
		})
	}
}

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
	var rnd = templates.HTMLRenderer()
	return func(next http.Handler) http.Handler {
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
					sessionStore := session.GetSession(req)
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

					if setting.RunMode != "prod" {
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

// BaseRoute creates a route
func BaseRoute() *web.Route {
	r := web.NewRoute()
	r.Use(middleware.RealIP)
	if !setting.DisableRouterLog && setting.RouterLogLevel != log.NONE {
		if log.GetLogger("router").GetLevel() <= setting.RouterLogLevel {
			r.Use(LoggerHandler(setting.RouterLogLevel))
		}
	}

	r.Use(session.Sessioner(session.Options{
		Provider:       setting.SessionConfig.Provider,
		ProviderConfig: setting.SessionConfig.ProviderConfig,
		CookieName:     setting.SessionConfig.CookieName,
		CookiePath:     setting.SessionConfig.CookiePath,
		Gclifetime:     setting.SessionConfig.Gclifetime,
		Maxlifetime:    setting.SessionConfig.Maxlifetime,
		Secure:         setting.SessionConfig.Secure,
		Domain:         setting.SessionConfig.Domain,
	}))

	r.Use(Recovery())
	if setting.EnableAccessLog {
		r.Use(accessLogger())
	}

	r.Use(public.Custom(
		&public.Options{
			SkipLogging: setting.DisableRouterLog,
		},
	))
	r.Use(public.Static(
		&public.Options{
			Directory:   path.Join(setting.StaticRootPath, "public"),
			SkipLogging: setting.DisableRouterLog,
		},
	))

	return r
}
