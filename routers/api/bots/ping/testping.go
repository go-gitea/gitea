// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ping

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	pingv1 "gitea.com/gitea/proto-go/ping/v1"
	"gitea.com/gitea/proto-go/ping/v1/pingv1connect"

	"github.com/bufbuild/connect-go"
	"github.com/stretchr/testify/assert"
)

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
			result, err := client.Ping(context.Background(), connect.NewRequest(&pingv1.PingRequest{
				Data: "foobar",
			}))
			assert.NoError(t, err)
			assert.Equal(t, "Hello, foobar!", result.Msg.Data)
		}
	})
}
