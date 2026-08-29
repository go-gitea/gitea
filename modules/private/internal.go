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

// internalAPIConnectionIsLocal reports whether the internal API transport connects to a local target,
// where the self-signed local certificate cannot be verified so skipping verification is safe. It mirrors
// what dialContextInternalAPI actually dials: a unix socket whenever Protocol is HTTPUnix (always local,
// whatever LOCAL_ROOT_URL says), otherwise the LOCAL_ROOT_URL host directly. A non-loopback LOCAL_ROOT_URL
// is a real network hop, so its certificate must be verified, else the internal token can be MITM'd. An
// unparseable LOCAL_ROOT_URL is a hard misconfiguration and fails closed (verify).
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
	return ip != nil && ip.IsLoopback()
}

var internalAPITransport = sync.OnceValue(func() http.RoundTripper {
	return &http.Transport{
		DialContext: dialContextInternalAPI,
		TLSClientConfig: &tls.Config{
			// Skip verification only for a local target (unix socket, or a loopback LOCAL_ROOT_URL), where the
			// self-signed local cert can't be verified anyway; a non-loopback LOCAL_ROOT_URL is a real network
			// hop and must be verified so the internal token can't be MITM'd. When verifying, Go's default
			// ServerName (the dialed LOCAL_ROOT_URL host) is already correct, so it is not overridden.
			InsecureSkipVerify: internalAPIConnectionIsLocal(setting.Protocol, setting.LocalURL),
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
