// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.dev/modules/json"
)

type swaggerResponseRef struct {
	Ref string `json:"$ref"`
}

type swaggerOperation struct {
	Responses map[string]swaggerResponseRef `json:"responses"`
}

type swaggerSpec struct {
	Paths map[string]map[string]swaggerOperation `json:"paths"`
}

func TestSwaggerIncludesAuthResponsesForUserKeyEndpoints(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "..", "templates", "swagger", "v1_json.tmpl")
	data, err := os.ReadFile(specPath)
	require.NoError(t, err)

	var spec swaggerSpec
	err = json.Unmarshal(data, &spec)
	require.NoError(t, err)

	for _, path := range []string{"/user/gpg_keys", "/user/gpg_keys/{id}", "/user/keys", "/user/keys/{id}"} {
		operations, ok := spec.Paths[path]
		require.True(t, ok, "expected swagger path %s to exist", path)

		for _, method := range []string{"get", "post", "delete"} {
			op, ok := operations[method]
			if !ok {
				continue
			}

			require.Contains(t, op.Responses, "401", "expected %s %s to document 401 responses", path, method)
			require.Equal(t, "#/responses/unauthorized", op.Responses["401"].Ref, "expected %s %s to reference unauthorized response", path, method)
			require.Contains(t, op.Responses, "403", "expected %s %s to document 403 responses", path, method)
			require.Equal(t, "#/responses/forbidden", op.Responses["403"].Ref, "expected %s %s to reference forbidden response", path, method)
		}
	}
}
