// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package ping

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pingv1 "gitea.dev/actionslib/ping/v1"
	"gitea.dev/actionslib/ping/v1/pingv1connect"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	_, handler := NewPingServiceHandler()
	MainServiceTest(t, handler)
}

func MainServiceTest(t *testing.T, h http.Handler) {
	t.Parallel()
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	connectClient := pingv1connect.NewPingServiceClient(connect.NewClient(connecthttp.NewTransport(
		server.Client(),
		server.URL,
	)))

	grpcClient := pingv1connect.NewPingServiceClient(connect.NewClient(connecthttp.NewTransport(
		server.Client(),
		server.URL,
		connecthttp.WithGRPC(),
	)))

	grpcWebClient := pingv1connect.NewPingServiceClient(connect.NewClient(connecthttp.NewTransport(
		server.Client(),
		server.URL,
		connecthttp.WithGRPCWeb(),
	)))

	clients := []pingv1connect.PingServiceClient{connectClient, grpcClient, grpcWebClient}
	t.Run("ping request", func(t *testing.T) {
		for _, client := range clients {
			result, err := client.Ping(t.Context(), &pingv1.PingRequest{
				Data: "foobar",
			})
			require.NoError(t, err)
			assert.Equal(t, "Hello, foobar!", result.Data)
		}
	})
}
