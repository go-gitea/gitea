// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/proxyprotocol"
	"code.gitea.io/gitea/modules/setting"
)

func newRequest(ctx context.Context, url, method, sourceIP string) *httplib.Request {
	if setting.InternalToken == "" {
		log.Fatal(`The INTERNAL_TOKEN setting is missing from the configuration file: %q.
Ensure you are running in the correct environment or set the correct configuration file with -c.`, setting.CustomConf)
	}
	return httplib.NewRequest(url, method).
		SetContext(ctx).
		Header("X-Real-IP", sourceIP).
		Header("Authorization", fmt.Sprintf("Bearer %s", setting.InternalToken))
}

// Response internal request response
type Response struct {
	Err string `json:"err"`
}

func decodeJSONError(resp *http.Response) *Response {
	var res Response
	err := json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		res.Err = err.Error()
	}
	return &res
}

func getClientIP() string {
	sshConnEnv := strings.TrimSpace(os.Getenv("SSH_CONNECTION"))
	if len(sshConnEnv) == 0 {
		return "127.0.0.1"
	}
	return strings.Fields(sshConnEnv)[0]
}

func newInternalRequest(ctx context.Context, url, method string) *httplib.Request {
	req := newRequest(ctx, url, method, getClientIP()).SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		ServerName:         setting.Domain,
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
	return req
}
