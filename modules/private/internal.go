// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/proxyprotocol"
	"gitea.dev/modules/setting"
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

// unspecified IPs dial the local host too, other targets are network hops that must be verified to protect INTERNAL_TOKEN
func internalAPIConnectionIsLocal(protocol setting.Scheme, localURL string) bool {
	if protocol == setting.HTTPUnix {
		return true
	}
	u, err := url.Parse(localURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

func internalAPITLSConfig(protocol setting.Scheme, localURL, domain string) *tls.Config {
	if internalAPIConnectionIsLocal(protocol, localURL) {
		// the ACME listener selects its certificate by SNI, so send the public domain instead of the local host
		return &tls.Config{InsecureSkipVerify: true, ServerName: domain}
	}
	return &tls.Config{}
}

var internalAPITransport = sync.OnceValue(func() http.RoundTripper {
	return &http.Transport{
		DialContext:     dialContextInternalAPI,
		TLSClientConfig: internalAPITLSConfig(setting.Protocol, setting.LocalURL, setting.AppDomain),
	}
})

func NewInternalRequest(ctx context.Context, url, method string) *httplib.ClientRequest {
	if setting.InternalToken == "" {
		log.Fatal(`The INTERNAL_TOKEN setting is missing from the configuration file: %q.
Ensure you are running in the correct environment or set the correct configuration file with -c.`, setting.CustomConf)
	}

	if !strings.HasPrefix(url, setting.LocalURL) {
		log.Fatal("Invalid internal request URL: %q", url)
	}

	return httplib.NewClientRequest(method, url).
		SetContext(ctx).
		SetTransport(internalAPITransport()).
		Header("X-Real-IP", getClientIP()).
		Header("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
}

func newInternalRequestAPI(ctx context.Context, url, method string, body ...any) *httplib.ClientRequest {
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
