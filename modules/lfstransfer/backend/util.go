// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/charmbracelet/git-lfs-transfer/transfer"
)

// HTTP headers
const (
	headerAccept            = "Accept"
	headerAuthorization     = "Authorization"
	headerGiteaInternalAuth = "X-Gitea-Internal-Auth"
	headerContentType       = "Content-Type"
	headerContentLength     = "Content-Length"
)

// MIME types
const (
	mimeGitLFS      = "application/vnd.git-lfs+json"
	mimeOctetStream = "application/octet-stream"
)

// SSH protocol action keys
const (
	actionDownload = "download"
	actionUpload   = "upload"
	actionVerify   = "verify"
)

// SSH protocol argument keys
const (
	argCursor    = "cursor"
	argExpiresAt = "expires-at"
	argID        = "id"
	argLimit     = "limit"
	argPath      = "path"
	argRefname   = "refname"
	argToken     = "token"
	argTransfer  = "transfer"
)

// Default username constants
const (
	userSelf    = "(self)"
	userUnknown = "(unknown)"
)

// Operations enum
const (
	opDownload = iota + 1
	opUpload
)

var opMap = map[string]int{
	"download": opDownload,
	"upload":   opUpload,
}

var ErrMissingID = fmt.Errorf("%w: missing id arg", transfer.ErrMissingData)

func statusCodeToErr(code int) error {
	switch code {
	case http.StatusBadRequest:
		return transfer.ErrParseError
	case http.StatusConflict:
		return transfer.ErrConflict
	case http.StatusForbidden:
		return transfer.ErrForbidden
	case http.StatusNotFound:
		return transfer.ErrNotFound
	case http.StatusUnauthorized:
		return transfer.ErrUnauthorized
	default:
		return fmt.Errorf("server returned status %v: %v", code, http.StatusText(code))
	}
}

func toInternalLFSURL(s string) string {
	pos1 := strings.Index(s, "://")
	if pos1 == -1 {
		return ""
	}
	appSubURLWithSlash := setting.AppSubURL + "/"
	pos2 := strings.Index(s[pos1+3:], appSubURLWithSlash)
	if pos2 == -1 {
		return ""
	}
	routePath := s[pos1+3+pos2+len(appSubURLWithSlash):]
	fields := strings.SplitN(routePath, "/", 3)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "info/lfs") {
		return ""
	}
	return setting.LocalURL + "api/internal/repo/" + routePath
}

func isInternalLFSURL(s string) bool {
	if !strings.HasPrefix(s, setting.LocalURL) {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	routePath := util.PathJoinRelX(u.Path)
	subRoutePath, cut := strings.CutPrefix(routePath, "api/internal/repo/")
	if !cut {
		return false
	}
	fields := strings.SplitN(subRoutePath, "/", 3)
	if len(fields) < 3 || !strings.HasPrefix(fields[2], "info/lfs") {
		return false
	}
	return true
}

func newInternalRequestLFS(ctx context.Context, internalURL, method string, headers map[string]string, body any) *httplib.Request {
	if !isInternalLFSURL(internalURL) {
		return nil
	}
	req := private.NewInternalRequest(ctx, internalURL, method)
	for k, v := range headers {
		req.Header(k, v)
	}
	switch body := body.(type) {
	case nil: // do nothing
	case []byte:
		req.Body(body) // []byte
	case io.Reader:
		req.Body(body) // io.Reader or io.ReadCloser
	default:
		panic(fmt.Sprintf("unsupported request body type %T", body))
	}
	return req
}
