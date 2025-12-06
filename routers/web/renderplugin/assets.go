// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package renderplugin

import (
	"net/http"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/renderplugin"
	"code.gitea.io/gitea/modules/setting"
	plugin_service "code.gitea.io/gitea/services/renderplugin"
)

// AssetsHandler returns an http.Handler that serves plugin metadata and static files.
func AssetsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		prefix := setting.AppSubURL + "/assets/render-plugins/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(r.URL.Path, prefix)
		rel = strings.TrimLeft(rel, "/")
		if rel == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if rel == "index.json" {
			serveMetadata(w, r)
			return
		}
		parts := strings.SplitN(rel, "/", 2)
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		clean := path.Clean("/" + parts[1])
		if clean == "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		clean = strings.TrimPrefix(clean, "/")
		if strings.HasPrefix(clean, "../") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		objectPath := renderplugin.ObjectPath(parts[0], clean)
		obj, err := renderplugin.Storage().Open(objectPath)
		if err != nil {
			if os.IsNotExist(err) {
				w.WriteHeader(http.StatusNotFound)
			} else {
				log.Error("Unable to open render plugin asset %s: %v", objectPath, err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		defer obj.Close()
		info, err := obj.Stat()
		if err != nil {
			log.Error("Unable to stat render plugin asset %s: %v", objectPath, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.ServeContent(w, r, path.Base(clean), info.ModTime(), obj)
	})
}

func serveMetadata(w http.ResponseWriter, r *http.Request) {
	meta, err := plugin_service.BuildMetadata(r.Context())
	if err != nil {
		log.Error("Unable to build render plugin metadata: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		log.Error("Failed to encode render plugin metadata: %v", err)
	}
}
