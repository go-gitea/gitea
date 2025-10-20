// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/proxyprotocol"
	"code.gitea.io/gitea/modules/setting"
)

// Response is used for internal request response (for user message and error message)
type Response struct {
	Err     string `json:"err,omitempty"`      // server-side error log message, it won't be exposed to end users
	UserMsg string `json:"user_msg,omitempty"` // meaningful error message for end users, it will be shown in git client's output.
}

func getClientIP() string {
	sshConnEnv := strings.TrimSpace(os.Getenv("SSH_CONNECTION"))
	if len(sshConnEnv) == 0 {
		return "127.0.0.1"
	}
	return strings.Fields(sshConnEnv)[0]
}

func dialContextInternalAPI(ctx context.Context, network, address string) (conn net.Conn, err error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	if setting.Protocol == setting.HTTPUnix {
		conn, err = d.DialContext(ctx, "unix", setting.HTTPAddr)
	} else {
		conn, err = d.DialContext(ctx, network, address)
	}
	if err != nil {
		return nil, err
	}
	if setting.LocalUseProxyProtocol {
		if err = proxyprotocol.WriteLocalHeader(conn); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

var internalAPITransport = sync.OnceValue(func() http.RoundTripper {
	return &http.Transport{
		DialContext: dialContextInternalAPI,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         setting.Domain,
		},
	}
})

func NewInternalRequest(ctx context.Context, url, method string) *httplib.Request {
	if setting.InternalToken == "" {
		log.Fatal(`The INTERNAL_TOKEN setting is missing from the configuration file: %q.
Ensure you are running in the correct environment or set the correct configuration file with -c.`, setting.CustomConf)
	}

	if !strings.HasPrefix(url, setting.LocalURL) {
		log.Fatal("Invalid internal request URL: %q", url)
	}

	return httplib.NewRequest(url, method).
		SetContext(ctx).
		SetTransport(internalAPITransport()).
		Header("X-Real-IP", getClientIP()).
		Header("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
}

func newInternalRequestAPI(ctx context.Context, url, method string, body ...any) *httplib.Request {
	req := NewInternalRequest(ctx, url, method)
	if len(body) == 1 {
		req.Header("Content-Type", "application/json")
		jsonBytes, _ := json.Marshal(body[0])
		req.Body(jsonBytes)
	} else if len(body) > 1 {
		log.Fatal("Too many arguments for newInternalRequestAPI")
	}

	req.SetReadWriteTimeout(60 * time.Second)
	return req
}
