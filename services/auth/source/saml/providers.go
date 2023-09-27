// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/httplib"
)

// Providers is list of known/available providers.
type Providers map[string]Source

var providers = Providers{}

func readIdentityProviderMetadata(ctx context.Context, source *Source) ([]byte, error) {
	if source.IdentityProviderMetadata != "" {
		return []byte(source.IdentityProviderMetadata), nil
	}

	req := httplib.NewRequest(source.IdentityProviderMetadataURL, "GET")
	req.SetTimeout(20*time.Second, time.Minute)
	resp, err := req.Response()
	if err != nil {
		return nil, fmt.Errorf("Unable to contact gitea: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
