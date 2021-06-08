// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"

	"code.gitea.io/gitea/models"
)

// Ensure the struct implements the interface.
var (
	_ Auth = &Group{}
)

// Group implements the Auth interface with serval Auth.
type Group struct {
	methods []Auth
}

// NewGroup creates a new auth group
func NewGroup(methods ...Auth) *Group {
	return &Group{
		methods: methods,
	}
}

// Name represents the name of auth method
func (b *Group) Name() string {
	return "group"
}

// Init does nothing as the Basic implementation does not need to allocate any resources
func (b *Group) Init() error {
	for _, m := range b.methods {
		if err := m.Init(); err != nil {
			return err
		}
	}
	return nil
}

// Free does nothing as the Basic implementation does not have to release any resources
func (b *Group) Free() error {
	for _, m := range b.methods {
		if err := m.Free(); err != nil {
			return err
		}
	}
	return nil
}

// IsEnabled returns true as this plugin is enabled by default and its not possible to disable
// it from settings.
func (b *Group) IsEnabled() bool {
	return true
}

// VerifyAuthData extracts and validates
func (b *Group) VerifyAuthData(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	if !models.HasEngine {
		return nil
	}

	// Try to sign in with each of the enabled plugins
	for _, ssoMethod := range b.methods {
		if !ssoMethod.IsEnabled() {
			continue
		}
		user := ssoMethod.VerifyAuthData(req, w, store, sess)
		if user != nil {
			if store.GetData()["AuthedMethod"] == nil {
				store.GetData()["AuthedMethod"] = ssoMethod.Name()
			}
			return user
		}
	}

	return nil
}
