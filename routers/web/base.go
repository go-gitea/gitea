// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/routing"
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

				http.Redirect(w, req, u.String(), http.StatusTemporaryRedirect)
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
