// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package misc

import (
	"net/http"
	"path"
	"strconv"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/context"
)

func SSHInfo(rw http.ResponseWriter, req *http.Request) {
	if !git.DefaultFeatures().SupportProcReceive {
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	rw.Header().Set("content-type", "text/json;charset=UTF-8")
	_, err := rw.Write([]byte(`{"type":"agit","version":1}`))
	if err != nil {
		log.Error("fail to write result: err: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	rw.WriteHeader(http.StatusOK)
}

func DummyOK(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func RobotsTxt(w http.ResponseWriter, req *http.Request) {
	robotsTxt := util.FilePathJoinAbs(setting.CustomPath, "public/robots.txt")
	if ok, _ := util.IsExist(robotsTxt); !ok {
		robotsTxt = util.FilePathJoinAbs(setting.CustomPath, "robots.txt") // the legacy "robots.txt"
	}
	httpcache.SetCacheControlInHeader(w.Header(), httpcache.CacheControlForPublicStatic())
	http.ServeFile(w, req, robotsTxt)
}

func StaticRedirect(target string) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, path.Join(setting.StaticURLPrefix, target), http.StatusMovedPermanently)
	}
}

func WebBannerDismiss(ctx *context.Context) {
	_, rev, _ := setting.Config().Instance.WebBanner.ValueRevision(ctx)
	middleware.SetSiteCookie(ctx.Resp, middleware.CookieWebBannerDismissed, strconv.Itoa(rev), 48*3600)
	ctx.JSONOK()
}
