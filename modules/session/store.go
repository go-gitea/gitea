// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package session

import (
	"net/http"

	"code.gitea.io/gitea/modules/setting"

	"gitea.com/go-chi/session"
)

type RawStore = session.RawStore

type Store interface {
	RawStore
	Destroy(http.ResponseWriter, *http.Request) error
}

type mockStoreContextKeyStruct struct{}

var MockStoreContextKey = mockStoreContextKeyStruct{}

// RegenerateSession regenerates the underlying session and returns the new store
func RegenerateSession(resp http.ResponseWriter, req *http.Request) (Store, error) {
	for _, f := range BeforeRegenerateSession {
		f(resp, req)
	}
	if setting.IsInTesting {
		if store := req.Context().Value(MockStoreContextKey); store != nil {
			return store.(Store), nil
		}
	}
	return session.RegenerateSession(resp, req)
}

func GetContextSession(req *http.Request) Store {
	if setting.IsInTesting {
		if store := req.Context().Value(MockStoreContextKey); store != nil {
			return store.(Store)
		}
	}
	return session.GetSession(req)
}

// BeforeRegenerateSession is a list of functions that are called before a session is regenerated.
var BeforeRegenerateSession []func(http.ResponseWriter, *http.Request)
