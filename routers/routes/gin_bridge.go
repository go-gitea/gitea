// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/setting"

	"github.com/gin-gonic/gin"
	macaron "gopkg.in/macaron.v1"
)

func ginBridgeMiddleware() macaron.Handler {
	g := gin.Default()
	if setting.ProdMode {
		gin.SetMode(gin.ReleaseMode)
	}

	// for health check
	g.HEAD("/", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusOK)
	})

	// robots.txt
	if setting.HasRobotsTxt {
		g.GET("/robots.txt", func(ctx *gin.Context) {
			ctx.File(path.Join(setting.CustomPath, "robots.txt"))
		})
	}

	routes := g.Routes()
	isGinRoutePath := func(method, p string) bool {
		for _, r := range routes {
			if strings.EqualFold(p, r.Path) && strings.EqualFold(method, r.Method) {
				return true
			}
		}
		return false
	}

	return func(ctx *macaron.Context) {
		if isGinRoutePath(ctx.Req.Method, ctx.Req.Request.RequestURI) {
			g.ServeHTTP(ctx.Resp, ctx.Req.Request)
		}
	}
}
