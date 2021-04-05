// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/url"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

func setupFlash(ctx *Context) {

	// Get the temporary flash cookie from the request
	flashCookie := ctx.GetCookie("macaron_flash")

	// Parse its data
	vals, _ := url.ParseQuery(flashCookie)
	if len(vals) > 0 {
		// If there is content then create a flash struct containing this data
		f := &middleware.Flash{
			DataStore:  ctx,
			Values:     vals,
			ErrorMsg:   vals.Get("error"),
			SuccessMsg: vals.Get("success"),
			InfoMsg:    vals.Get("info"),
			WarningMsg: vals.Get("warning"),
		}
		// And stick it in the context datastore
		ctx.Data["Flash"] = f
	}

	// Now create a new empty Flash struct for this response
	f := &middleware.Flash{
		DataStore:  ctx,
		Values:     url.Values{},
		ErrorMsg:   "",
		WarningMsg: "",
		InfoMsg:    "",
		SuccessMsg: "",
	}

	// Add a handler to write/delete the cookie before the response is written
	ctx.Resp.Before(func(resp ResponseWriter) {
		if flash := f.Encode(); len(flash) > 0 {
			// If our flash object contains data - then save it to the cookie
			middleware.SetCookie(resp, "macaron_flash", flash, 0,
				setting.SessionConfig.CookiePath,
				middleware.Domain(setting.SessionConfig.Domain),
				middleware.HTTPOnly(true),
				middleware.Secure(setting.SessionConfig.Secure),
				middleware.SameSite(setting.SessionConfig.SameSite),
			)
			return
		}
		// Otherwise delete the flash cookie
		middleware.SetCookie(ctx.Resp, "macaron_flash", "", -1,
			setting.SessionConfig.CookiePath,
			middleware.Domain(setting.SessionConfig.Domain),
			middleware.HTTPOnly(true),
			middleware.Secure(setting.SessionConfig.Secure),
			middleware.SameSite(setting.SessionConfig.SameSite),
		)
	})

	// Save the new empty Flash as ctx.Flash
	ctx.Flash = f
}
