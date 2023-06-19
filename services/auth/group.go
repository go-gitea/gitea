// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"net/http"
	"reflect"
	"strings"

	user_model "code.gitea.io/gitea/models/user"
)

// Ensure the struct implements the interface.
var (
	_ Method = &Group{}
)

// Group implements the Auth interface with serval Auth.
type Group struct {
	methods []Method
}

// NewGroup creates a new auth group
func NewGroup(methods ...Method) *Group {
	return &Group{
		methods: methods,
	}
}

// Add adds a new method to group
func (b *Group) Add(method Method) {
	b.methods = append(b.methods, method)
}

// Name returns group's methods name
func (b *Group) Name() string {
	names := make([]string, 0, len(b.methods))
	for _, m := range b.methods {
		if n, ok := m.(Named); ok {
			names = append(names, n.Name())
		} else {
			names = append(names, reflect.TypeOf(m).Elem().Name())
		}
	}
	return strings.Join(names, ",")
}

// Verify extracts and validates
func (b *Group) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error) {
	// Try to sign in with each of the enabled plugins
	for _, ssoMethod := range b.methods {
		user, err := ssoMethod.Verify(req, w, store, sess)
		if err != nil {
			return nil, err
		}

		if user != nil {
			if store.GetData()["AuthedMethod"] == nil {
				if named, ok := ssoMethod.(Named); ok {
					store.GetData()["AuthedMethod"] = named.Name()
				}
			}
			return user, nil
		}
	}

	return nil, nil
}
