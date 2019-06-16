// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"bytes"
	"fmt"
	"net/http"
	"path"
	"text/template"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gin-gonic/gin"
)

type routerLoggerOptions struct {
	Ctx            *gin.Context
	Identity       *string
	Start          *time.Time
	ResponseWriter gin.ResponseWriter
}

// SignedUserName returns signed user's name via context
// FIXME currently no any data stored on gin.Context but macaron.Context, so this will
// return "" before we remove macaron totally
func SignedUserName(ctx *gin.Context) string {
	if v, ok := ctx.Get("SignedUserName"); !ok {
		return ""
	} else {
		return v.(string)
	}
}

func setupAccessLogger(g *gin.Engine) {
	logger := log.GetLogger("access")

	logTemplate, _ := template.New("log").Parse(setting.AccessLogTemplate)
	g.Use(func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()
		identity := "-"
		if val := SignedUserName(ctx); val != "" {
			identity = val
		}
		rw := ctx.Writer

		buf := bytes.NewBuffer([]byte{})
		err := logTemplate.Execute(buf, routerLoggerOptions{
			Ctx:            ctx,
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

// RouterHandler is a macaron handler that will log the routing to the default gitea log
func RouterHandler(level log.Level) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()

		_ = log.GetLogger("router").Log(0, level, "Started %s %s for %s", log.ColoredMethod(ctx.Request.Method), ctx.Request.RequestURI, ctx.Request.RemoteAddr)

		rw := ctx.Writer
		ctx.Next()

		status := rw.Status()
		_ = log.GetLogger("router").Log(0, level, "Completed %s %s %v %s in %v", log.ColoredMethod(ctx.Request.Method), ctx.Request.RequestURI, log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(rw.Status())), log.ColoredTime(time.Since(start)))
	}
}

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// Although similar to macaron.Recovery() the main difference is that this error will be created
// with the gitea 500 page.
func Recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				combinedErr := fmt.Errorf("%s\n%s", err, string(log.Stack(2)))
				ctx.String(500, "PANIC: %v", combinedErr)
			}
		}()

		ctx.Next()
	}
}

// NewGin creates a gin Engine
func NewGin() *gin.Engine {
	g := gin.New()
	if !setting.DisableRouterLog {
		g.Use(RouterHandler(setting.RouterLogLevel))
	}
	g.Use(Recovery())
	if setting.EnableAccessLog {
		setupAccessLogger(g)
	}
	if setting.ProdMode {
		gin.SetMode(gin.ReleaseMode)
	}
	return g
}

// RegisterRoutes registers gin routes
func RegisterRoutes(g *gin.Engine) {
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

	m := NewMacaron()
	RegisterMacaronRoutes(m)

	g.Use(func(ctx *gin.Context) {
		m.ServeHTTP(ctx.Writer, ctx.Request)
	})
}
