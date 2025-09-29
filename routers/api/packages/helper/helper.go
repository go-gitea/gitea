// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// ProcessErrorForUser logs the error and returns a user-error message for the end user.
// If the status is http.StatusInternalServerError, the message is stripped for non-admin users in production.
func ProcessErrorForUser(ctx *context.Context, status int, errObj any) string {
	var message string
	if err, ok := errObj.(error); ok {
		message = err.Error()
	} else if errObj != nil {
		message = fmt.Sprint(errObj)
	}

	if status == http.StatusInternalServerError {
		log.Log(2, log.ERROR, "Package registry API internal error: %d %s", status, message)
		if setting.IsProd && (ctx.Doer == nil || !ctx.Doer.IsAdmin) {
			message = "internal server error"
		}
		return message
	}

	log.Log(2, log.DEBUG, "Package registry API user error: %d %s", status, message)
	return message
}

// ServePackageFile the content of the package file
// If the url is set it will redirect the request, otherwise the content is copied to the response.
func ServePackageFile(ctx *context.Context, s io.ReadSeekCloser, u *url.URL, pf *packages_model.PackageFile, forceOpts ...*context.ServeHeaderOptions) {
	if u != nil {
		ctx.Redirect(u.String())
		return
	}

	defer s.Close()

	var opts *context.ServeHeaderOptions
	if len(forceOpts) > 0 {
		opts = forceOpts[0]
	} else {
		opts = &context.ServeHeaderOptions{
			Filename:     pf.Name,
			LastModified: pf.CreatedUnix.AsLocalTime(),
		}
	}

	ctx.ServeContent(s, opts)
}
