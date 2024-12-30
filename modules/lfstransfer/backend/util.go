// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package backend

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/proxyprotocol"
	"code.gitea.io/gitea/modules/setting"

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

func newInternalRequest(ctx context.Context, url, method string, headers map[string]string, body []byte) *httplib.Request {
	req := httplib.NewRequest(url, method).
		SetContext(ctx).
		SetTimeout(10*time.Second, 60*time.Second).
		SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: true,
		})

	if setting.Protocol == setting.HTTPUnix {
		req.SetTransport(&http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				conn, err := d.DialContext(ctx, "unix", setting.HTTPAddr)
				if err != nil {
					return conn, err
				}
				if setting.LocalUseProxyProtocol {
					if err = proxyprotocol.WriteLocalHeader(conn); err != nil {
						_ = conn.Close()
						return nil, err
					}
				}
				return conn, err
			},
		})
	} else if setting.LocalUseProxyProtocol {
		req.SetTransport(&http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				var d net.Dialer
				conn, err := d.DialContext(ctx, network, address)
				if err != nil {
					return conn, err
				}
				if err = proxyprotocol.WriteLocalHeader(conn); err != nil {
					_ = conn.Close()
					return nil, err
				}
				return conn, err
			},
		})
	}

	for k, v := range headers {
		req.Header(k, v)
	}

	req.Body(body)

	return req
}
