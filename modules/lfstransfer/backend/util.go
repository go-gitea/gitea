// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/private"

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
	opNone = iota
	opDownload
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

func newInternalRequestLFS(ctx context.Context, url, method string, headers map[string]string, body any) *httplib.Request {
	req := private.NewInternalRequest(ctx, url, method)
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
