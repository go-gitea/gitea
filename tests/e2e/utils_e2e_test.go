// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package e2e

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func onGiteaRunTB(t testing.TB, callback func(testing.TB, *url.URL), prepare ...bool) {
	if len(prepare) == 0 || prepare[0] {
		defer tests.PrepareTestEnv(t, 1)()
	}
	s := http.Server{
		Handler: testE2eWebRoutes,
	}

	u, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	listener, err := net.Listen("tcp", u.Host)
	i := 0
	for err != nil && i <= 10 {
		time.Sleep(100 * time.Millisecond)
		listener, err = net.Listen("tcp", u.Host)
		i++
	}
	assert.NoError(t, err)
	u.Host = listener.Addr().String()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)
	// Started by config go ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)

	callback(t, u)
}

func onGiteaRun(t *testing.T, callback func(*testing.T, *url.URL), prepare ...bool) {
	onGiteaRunTB(t, func(t testing.TB, u *url.URL) {
		callback(t.(*testing.T), u)
	}, prepare...)
}
