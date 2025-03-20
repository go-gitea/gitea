// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"

	"code.gitea.io/gitea/modules/httplib"
)

// FetchRedirectDelegate helps the "fetch" requests to redirect to the correct location
func FetchRedirectDelegate(resp http.ResponseWriter, req *http.Request) {
	// When use "fetch" to post requests and the response is a redirect, browser's "location.href = uri" has limitations.
	// 1. change "location" from old "/foo" to new "/foo#hash", the browser will not reload the page.
	// 2. when use "window.reload()", the hash is not respected, the newly loaded page won't scroll to the hash target.
	// The typical page is "issue comment" page. The backend responds "/owner/repo/issues/1#comment-2",
	// then frontend needs this delegate to redirect to the new location with hash correctly.
	redirect := req.PostFormValue("redirect")
	if !httplib.IsCurrentGiteaSiteURL(req.Context(), redirect) {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.Header().Add("Location", redirect)
	resp.WriteHeader(http.StatusSeeOther)
}
