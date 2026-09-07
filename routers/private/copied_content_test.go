// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gitea_context "gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopiedContentJSON(t *testing.T) {
	const valid = `{"request_id":"one","user_id":2,"destination":{"repository_id":16,"branch":"main","commit":"abc","parent":null},"files":[{"source_repository_id":1,"source_commit":"abc","source_path":"a","destination_path":"b","representation":"lfs"}]}`
	for name, body := range map[string]string{
		"closing_bracket":  valid + "]",
		"closing_brace":    valid + "}",
		"second_value":     valid + "{}",
		"case_alias":       strings.Replace(valid, `"request_id":"one"`, `"request_id":"one","REQUEST_ID":"two"`, 1),
		"unicode_alias":    strings.Replace(valid, `"user_id"`, `"uſer_id"`, 1),
		"noncanonical_key": strings.Replace(valid, `"user_id"`, `"User_ID"`, 1),
		"duplicate_nested": strings.Replace(valid, `"parent":null`, `"parent":null,"parent":"abc"`, 1),
		"unknown_field":    strings.Replace(valid, `"parent":null`, `"parent":null,"unexpected":0`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			var req repo_service.FinalizeCopiedContentRequest
			require.Error(t, decodeCopiedContentJSON([]byte(body), &req))
		})
	}
	var req repo_service.FinalizeCopiedContentRequest
	require.NoError(t, decodeCopiedContentJSON([]byte(valid+" \n\t"), &req))
	assert.Nil(t, req.Destination.Parent)
	assert.True(t, jsonFieldPresent([]byte(valid), "destination", "parent"))
	assert.False(t, jsonFieldPresent([]byte(strings.Replace(valid, `,"parent":null`, "", 1)), "destination", "parent"))
}

func TestCopiedContentBodyReadDeadline(t *testing.T) {
	// Use a real connection: cancelling a request context alone cannot unblock Read.
	result := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()
		_, err := readCopiedContentBody(ctx, gitea_context.WrapResponseWriter(w), r)
		result <- err
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	require.NoError(t, err)
	defer conn.Close()
	_, err = fmt.Fprintf(conn, "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 100\r\n\r\n{")
	require.NoError(t, err)
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("stalled body outlived request deadline")
	}
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer response.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
}
