// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package session

import (
	"net/http"

	"gitea.com/go-chi/session"
)

// Store represents a session store
type Store interface {
	Get(interface{}) interface{}
	Set(interface{}, interface{}) error
	Delete(interface{}) error
}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	s, err := session.RegenerateSession(resp, req)
	return s, err
}

// EmptyStore represents an empty store
type EmptyStore struct{}

// NewEmptyStore returns an EmptyStore
func NewEmptyStore() *EmptyStore {
	return &EmptyStore{}
}

func (EmptyStore) Get(interface{}) interface{} {
	return nil
}

func (EmptyStore) Set(interface{}, interface{}) error {
	return nil
}

func (EmptyStore) Delete(interface{}) error {
	return nil
}
