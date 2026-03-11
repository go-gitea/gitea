// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"errors"
	"fmt"
	"image/png"
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/assetfs"
	"code.gitea.io/gitea/modules/avatar"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/routing"
)

func avatarStorageHandler(storageSetting *setting.Storage, prefix string, objStore storage.ObjectStorage) http.HandlerFunc {
	prefix = strings.Trim(prefix, "/")
	funcInfo := routing.GetFuncInfo(avatarStorageHandler, prefix)
	exeModTime := assetfs.GetExecutableModTime()
	fallbackEtag := fmt.Sprintf(`"avatar-%s"`, exeModTime.Format("20060102150405"))

	handleError := func(w http.ResponseWriter, req *http.Request, avatarPath string, err error) bool {
		if err == nil {
			return false
		}

		if errors.Is(err, os.ErrNotExist) || errors.Is(err, util.ErrNotExist) {
			// if avatar doesn't exist, generate a random one and serve it with proper cache control headers
			w.Header().Set("Content-Type", "image/png")
			if !httpcache.HandleGenericETagPublicCache(req, w, fallbackEtag, &exeModTime) {
				if req.Method == http.MethodGet {
					img := avatar.RandomImageWithSize(96, []byte(avatarPath))
					_ = png.Encode(w, img)
				} // else: for HEAD request, just return the headers without body
			}
		} else {
			// for internal errors, log the error and return 500
			log.Error("Error when serving avatar %s: %s", req.URL.Path, err)
			http.Error(w, "unable to serve avatar image", http.StatusInternalServerError)
		}
		return true
	}

	return func(w http.ResponseWriter, req *http.Request) {
		defer routing.RecordFuncInfo(req.Context(), funcInfo)()

		avatarPath, ok := strings.CutPrefix(req.URL.Path, "/"+prefix+"/")
		if !ok {
			http.Error(w, "invalid avatar path", http.StatusBadRequest)
			return
		}
		avatarPath = util.PathJoinRelX(avatarPath)
		if avatarPath == "" || avatarPath == "." {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if storageSetting.ServeDirect() {
			// Old logic: no check for existence by Stat, so old code's "errors.Is(err, os.ErrNotExist)" didn't work.
			// So in theory, it doesn't work with the non-existing avatar fallback, it just gets the URL and redirects to it.
			// Checking "stat" requires one more request to the storage, which is inefficient.
			// Workaround: disable "SERVE_DIRECT". Leave the problem to the future.
			u, err := objStore.URL(avatarPath, path.Base(avatarPath), req.Method, nil)
			if handleError(w, req, avatarPath, err) {
				return
			}
			http.Redirect(w, req, u.String(), http.StatusTemporaryRedirect)
			return
		}

		fr, err := objStore.Open(avatarPath)
		if handleError(w, req, avatarPath, err) {
			return
		}
		defer fr.Close()

		fi, err := fr.Stat()
		if handleError(w, req, avatarPath, err) {
			return
		}

		httpcache.SetCacheControlInHeader(w.Header(), httpcache.CacheControlForPublicStatic())
		http.ServeContent(w, req, path.Base(avatarPath), fi.ModTime(), fr)
	}
}
