// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"net/http"

	"code.gitea.io/gitea/modules/log"
)

func grpcHandler(h http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("Got connection: %v", r.Proto)
		h.ServeHTTP(w, r)
	})
}
