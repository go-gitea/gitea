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

type emptyStore struct{}

// NewEmptyStore returns an emptyStore
func NewEmptyStore() *emptyStore {
	return &emptyStore{}
}

func (emptyStore) Get(interface{}) interface{} {
	return nil
}

func (emptyStore) Set(interface{}, interface{}) error {
	return nil
}

func (emptyStore) Delete(interface{}) error {
	return nil
}
