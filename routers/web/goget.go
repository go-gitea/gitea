// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"strings"

	auth_model "gitea.dev/models/auth"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
)

func goGet(ctx *context.Context) {
	if ctx.Req.Method != http.MethodGet || len(ctx.Req.URL.RawQuery) < 8 || ctx.FormString("go-get") != "1" {
		return
	}

	parts := strings.SplitN(ctx.Req.URL.EscapedPath(), "/", 4)

	if len(parts) < 3 {
		return
	}

	ownerName := parts[1]
	repoName := parts[2]

	// Quick responses appropriate go-get meta with status 200
	// regardless of if user have access to the repository,
	// or the repository does not exist at all.
	// This is particular a workaround for "go get" command which does not respect
	// .netrc file.

	trimmedRepoName := strings.TrimSuffix(repoName, ".git")

	if ownerName == "" || trimmedRepoName == "" {
		_, _ = ctx.Write([]byte(`<!doctype html>
<html>
	<body>
		invalid import path
	</body>
</html>
`))
		ctx.Status(http.StatusBadRequest)
		return
	}
	branchName := setting.Repository.DefaultBranch
	if repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName); err == nil {
		branchName = goGetDefaultBranch(ctx, repo)
	}
	prefix := setting.AppURL + path.Join(url.PathEscape(ownerName), url.PathEscape(repoName), "src", "branch", util.PathEscapeSegments(branchName))

	appURL, _ := url.Parse(setting.AppURL)

	insecure := ""
	if appURL.Scheme == string(setting.HTTP) {
		insecure = "--insecure "
	}

	goGetImport := context.ComposeGoGetImport(ctx, ownerName, trimmedRepoName)

	var cloneURL string
	if setting.Repository.GoGetCloneURLProtocol == "ssh" {
		cloneURL = repo_model.ComposeSSHCloneURL(ctx.Doer, ownerName, repoName)
	} else {
		cloneURL = repo_model.ComposeHTTPSCloneURL(ctx, ownerName, repoName)
	}
	goImportContent := fmt.Sprintf("%s git %s", goGetImport, cloneURL /*CloneLink*/)
	goSourceContent := fmt.Sprintf("%s _ %s %s", goGetImport, prefix+"{/dir}" /*GoDocDirectory*/, prefix+"{/dir}/{file}#L{line}" /*GoDocFile*/)
	goGetCli := fmt.Sprintf("go get %s%s", insecure, goGetImport)

	res := fmt.Sprintf(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="%s">
		<meta name="go-source" content="%s">
	</head>
	<body>
		%s
	</body>
</html>`, html.EscapeString(goImportContent), html.EscapeString(goSourceContent), html.EscapeString(goGetCli))

	ctx.RespHeader().Set("Content-Type", "text/html")
	_, _ = ctx.Write([]byte(res))
}

// goGetDefaultBranch returns the repository's real default branch only when the caller may genuinely
// reach it, otherwise the neutral instance default, so the meta response does not disclose the branch
// name (or the repo's existence) to callers who cannot see the repository.
func goGetDefaultBranch(ctx *context.Context, repo *repo_model.Repository) string {
	def := setting.Repository.DefaultBranch
	if len(repo.DefaultBranch) == 0 {
		return def
	}
	if err := repo.LoadOwner(ctx); err != nil || repo.Owner == nil {
		return def
	}
	// a token that was not granted repository read scope must not learn repository details, even when the
	// account behind it could read the repo through the web UI
	if !goGetTokenCanReadRepo(ctx) {
		return def
	}
	// a public-only token may only reach genuinely public resources (a public repo under a public owner)
	if context.TokenIsPublicOnly(ctx) && (repo.IsPrivate || !repo.Owner.Visibility.IsPublic()) {
		return def
	}
	// the caller must be able to read the code and see the owner: a limited/private owner hides its repos
	// from anonymous/non-member callers even when the repo itself is public
	perm, err := access_model.GetDoerRepoPermission(ctx, repo, ctx.Doer)
	if err != nil || !perm.CanRead(unit.TypeCode) || !user_model.IsUserVisibleToViewer(ctx, repo.Owner, ctx.Doer) {
		return def
	}
	return repo.DefaultBranch
}

// goGetTokenCanReadRepo reports whether the request may learn repository details. A non-token request
// always may; a token request may only when its scope grants repository read, so a PAT that was never
// scoped for repositories cannot disclose the branch even if its owner can read the repo.
func goGetTokenCanReadRepo(ctx *context.Context) bool {
	if ctx.Data["IsApiToken"] != true {
		return true
	}
	scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
	if !ok {
		return false
	}
	has, err := scope.HasScope(auth_model.AccessTokenScopeReadRepository)
	return err == nil && has
}
