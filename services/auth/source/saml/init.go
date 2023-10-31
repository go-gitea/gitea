// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	"context"
	"sync"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/log"
)

var samlRWMutex = sync.RWMutex{}

func Init(ctx context.Context) error {
	loginSources, _ := auth.GetActiveAuthProviderSources(ctx, auth.SAML)
	for _, source := range loginSources {
		samlSource, ok := source.Cfg.(*Source)
		if !ok {
			continue
		}
		err := samlSource.RegisterSource()
		if err != nil {
			log.Error("Unable to register source: %s due to Error: %v.", source.Name, err)
		}
	}
	return nil
}
