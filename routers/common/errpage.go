// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/modules/web/routing"
	"code.gitea.io/gitea/services/context"
)

const tplStatus500 templates.TplName = "status/500"

func renderServerErrorPage(w http.ResponseWriter, req *http.Request, respCode int, tmpl templates.TplName, ctxData map[string]any, plainMsg string) {
	acceptsHTML := false
	for _, part := range req.Header["Accept"] {
		if strings.Contains(part, "text/html") {
			acceptsHTML = true
			break
		}
	}

	httpcache.SetCacheControlInHeader(w.Header(), &httpcache.CacheControlOptions{NoTransform: true})
	w.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

	tmplCtx := context.NewTemplateContext(req.Context(), req)
	tmplCtx["Locale"] = middleware.Locale(w, req)

	w.WriteHeader(respCode)

	outBuf := &bytes.Buffer{}
	if acceptsHTML {
		err := templates.PageRenderer().HTML(outBuf, respCode, tmpl, ctxData, tmplCtx)
		if err != nil {
			_, _ = w.Write([]byte("Internal server error but failed to render error page template, please collect error logs and report to Gitea issue tracker"))
			return
		}
	} else {
		outBuf.WriteString(plainMsg)
	}
	_, _ = io.Copy(w, outBuf)
}

// RenderPanicErrorPage renders a 500 page, and it never panics
func RenderPanicErrorPage(w http.ResponseWriter, req *http.Request, err any) {
	combinedErr := fmt.Sprintf("%v\n%s", err, log.Stack(2))
	log.Error("PANIC: %s", combinedErr)

	defer func() {
		if err := recover(); err != nil {
			log.Error("Panic occurs again when rendering error page: %v. Stack:\n%s", err, log.Stack(2))
		}
	}()

	routing.UpdatePanicError(req.Context(), err)

	plainMsg := "Internal Server Error"
	ctxData := middleware.GetContextData(req.Context())
	// This recovery handler could be called without Gitea's web context, so we shouldn't touch that context too much.
	// Otherwise, the 500-page may cause new panics, eg: cache.GetContextWithData, it makes the developer&users couldn't find the original panic.
	user, _ := ctxData[middleware.ContextDataKeySignedUser].(*user_model.User)
	if !setting.IsProd || (user != nil && user.IsAdmin) {
		plainMsg = "PANIC: " + combinedErr
		ctxData["ErrorMsg"] = plainMsg
	}
	renderServerErrorPage(w, req, http.StatusInternalServerError, tplStatus500, ctxData, plainMsg)
}
