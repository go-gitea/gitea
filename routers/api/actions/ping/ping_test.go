// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ping

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pingv1 "code.gitea.io/actions-proto-go/ping/v1"
	"code.gitea.io/actions-proto-go/ping/v1/pingv1connect"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(pingv1connect.NewPingServiceHandler(
		&Service{},
	))
	MainServiceTest(t, mux)
}

func MainServiceTest(t *testing.T, h http.Handler) {
	t.Parallel()
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	connectClient := pingv1connect.NewPingServiceClient(
		server.Client(),
		server.URL,
	)

	grpcClient := pingv1connect.NewPingServiceClient(
		server.Client(),
		server.URL,
		connect.WithGRPC(),
	)

	grpcWebClient := pingv1connect.NewPingServiceClient(
		server.Client(),
		server.URL,
		connect.WithGRPCWeb(),
	)

	clients := []pingv1connect.PingServiceClient{connectClient, grpcClient, grpcWebClient}
	t.Run("ping request", func(t *testing.T) {
		for _, client := range clients {
			result, err := client.Ping(t.Context(), connect.NewRequest(&pingv1.PingRequest{
				Data: "foobar",
			}))
			require.NoError(t, err)
			assert.Equal(t, "Hello, foobar!", result.Msg.Data)
		}
	})
}
