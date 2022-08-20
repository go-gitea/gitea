// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"code.gitea.io/gitea/modules/web"
)

func Routes(r *web.Route) {
	// socket connection
	r.Get("/socket", socketServe)
	// runner service
	runnerServiceRoute(r)
	// ping service
	pingServiceRoute(r)
	// health service
	healthServiceRoute(r)
	// grpcv1 and v1alpha service
	grpcServiceRoute(r)
}
