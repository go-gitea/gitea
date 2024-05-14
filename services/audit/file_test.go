// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	audit_model "code.gitea.io/gitea/models/audit"
	repository_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"

	"github.com/stretchr/testify/assert"
)

func TestWriteEventAsJSON(t *testing.T) {
	r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
	m := &repository_model.PushMirror{ID: 4}
	doer := &user_model.User{ID: 2, Name: "Doer"}

	ctx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})

	e := buildEvent(
		ctx,
		audit_model.RepositoryMirrorPushAdd,
		doer,
		r,
		m,
		"Added push mirror for repository %s.",
		r.FullName(),
	)
	e.Time = time.Time{}

	sb := strings.Builder{}
	assert.NoError(t, WriteEventAsJSON(&sb, e))
	assert.Equal(
		t,
		`{"action":"repository:mirror:push:add","actor":{"type":"user","id":2,"display_name":"Doer"},"scope":{"type":"repository","id":3,"display_name":"TestUser/TestRepo"},"target":{"type":"push_mirror","id":4,"display_name":""},"message":"Added push mirror for repository TestUser/TestRepo.","time":"0001-01-01T00:00:00Z","ip_address":"127.0.0.1"}`+"\n",
		sb.String(),
	)
}
