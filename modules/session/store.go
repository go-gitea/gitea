// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"

	"gitea.com/go-chi/session"
)

// Store represents a session store
type Store interface {
	Get(any) any
	Set(any, any) error
	Delete(any) error
}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	s, err := session.RegenerateSession(resp, req)
	return s, err
}
