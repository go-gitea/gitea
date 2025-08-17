// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

func goGet(ctx *context.Context) {
	if ctx.Req.Method != http.MethodGet || len(ctx.Req.URL.RawQuery) < 8 || ctx.FormString("go-get") != "1" {
		return
	}

	parts := strings.SplitN(ctx.Req.URL.EscapedPath(), "/", 6)

	if len(parts) < 3 {
		return
	}
	var group string
	ownerName := parts[1]
	repoName := parts[2]
	if len(parts) > 4 {
		repoName = parts[4]
		group = parts[3]
	}

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
	gid, _ := strconv.ParseInt(group, 10, 64)
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName, gid)
	if err == nil && len(repo.DefaultBranch) > 0 {
		branchName = repo.DefaultBranch
	}
	prefix := setting.AppURL + url.PathEscape(ownerName)
	if group != "" {
		prefix = prefix + "/" + group
	}
	prefix = prefix + "/" + path.Join(url.PathEscape(repoName), "src", "branch", util.PathEscapeSegments(branchName))

	appURL, _ := url.Parse(setting.AppURL)

	insecure := ""
	if appURL.Scheme == string(setting.HTTP) {
		insecure = "--insecure "
	}

	goGetImport := context.ComposeGoGetImport(ctx, ownerName, trimmedRepoName)

	var cloneURL string
	gid, _ := strconv.ParseInt(group, 10, 64)
	if setting.Repository.GoGetCloneURLProtocol == "ssh" {
		cloneURL = repo_model.ComposeSSHCloneURL(ctx.Doer, ownerName, repoName, gid)
	} else {
		cloneURL = repo_model.ComposeHTTPSCloneURL(ctx, ownerName, repoName, gid)
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
