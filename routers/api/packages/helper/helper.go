// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	packages_model "code.gitea.io/gitea/models/packages"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// LogAndProcessError logs an error and calls a custom callback with the processed error message.
// If the error is an InternalServerError the message is stripped if the user is not an admin.
func LogAndProcessError(ctx *context.Context, status int, obj any, cb func(string)) {
	var message string
	if err, ok := obj.(error); ok {
		message = err.Error()
	} else if obj != nil {
		message = fmt.Sprintf("%s", obj)
	}
	if status == http.StatusInternalServerError {
		log.ErrorWithSkip(1, message)

		if setting.IsProd && (ctx.Doer == nil || !ctx.Doer.IsAdmin) {
			message = ""
		}
	} else {
		log.Debug(message)
	}

	if cb != nil {
		cb(message)
	}
}

// Serves the content of the package file
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

func TryConnectRepository(ctx *context.Context, packageID int64) error {
	headers := ctx.Req.Header["Package-Connection-Repository"]

	if len(headers) == 0 {
		return nil
	}

	if len(headers) != 1 {
		return util.NewInvalidArgumentErrorf("too many package repository connection headers")
	}

	repo, err := repo_model.GetRepositoryByName(ctx, ctx.Package.Owner.ID, headers[0])
	if err != nil {
		return err
	}

	perms, err := access_model.GetUserRepoPermission(ctx, repo, ctx.Doer)
	if err != nil {
		return err
	}

	if !perms.CanWrite(unit.TypePackages) {
		return util.NewPermissionDeniedErrorf("no permission to link package to repository: %s, or packages are disabled", repo.Name)
	}

	return packages_model.SetRepositoryLink(ctx, packageID, repo.ID)
}
