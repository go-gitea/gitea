// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"code.gitea.io/gitea/modules/haproxy"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"
)

func newRequest(url, method string) *httplib.Request {
	return httplib.NewRequest(url, method).Header("Authorization",
		fmt.Sprintf("Bearer %s", setting.InternalToken))
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

func newInternalRequest(url, method string) *httplib.Request {
	req := newRequest(url, method).SetTLSClientConfig(&tls.Config{
		InsecureSkipVerify: true,
		ServerName:         setting.Domain,
	})
	if setting.Protocol == setting.UnixSocket {
		req.SetTransport(&http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				conn, err := d.DialContext(ctx, "unix", setting.HTTPAddr)
				if err != nil {
					return conn, err
				}
				if setting.LocalUseProxyProtocol {
					if err = haproxy.WriteLocalProxyHeader(conn); err != nil {
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
				if err = haproxy.WriteLocalProxyHeader(conn); err != nil {
					_ = conn.Close()
					return nil, err
				}
				return conn, err
			},
		})
	}
	return req
}
