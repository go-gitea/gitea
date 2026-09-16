// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"

	"gitea.dev/modules/setting"

	"gitea.com/go-chi/session"
	"gitea.com/go-chi/session/core"
)

type RawStore = session.RawStore

type Store interface {
	RawStore
	Destroy(http.ResponseWriter, *http.Request) error
	// ListStoreByIndexer returns read-only session stores indexed by indexValue.
	ListStoreByIndexer(indexValue any) ([]core.RawStoreReadOnly, error)
	// DestroySessionByID destroys the session identified by sid and removes its index entry.
	DestroySessionByID(sid string, indexValue any) error
}

type mockStoreContextKeyStruct struct{}

var MockStoreContextKey = mockStoreContextKeyStruct{}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	for _, f := range BeforeRegenerateSession {
		f(resp, req)
	}
	if setting.IsInTesting {
		if store, ok := req.Context().Value(MockStoreContextKey).(Store); ok {
			return store, nil
		}
	}
	return session.RegenerateSession(resp, req)
}

func GetContextSession(req *http.Request) Store {
	if setting.IsInTesting {
		if store, ok := req.Context().Value(MockStoreContextKey).(Store); ok {
			return store
		}
	}
	return session.GetSession(req)
}

// BeforeRegenerateSession is a list of functions that are called before a session is regenerated.
var BeforeRegenerateSession []func(http.ResponseWriter, *http.Request)
