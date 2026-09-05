// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestWriteEventsAsJSON(t *testing.T) {
	r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
	m := &repository_model.PushMirror{ID: 4, RemoteAddress: "git@example.com:repo.git"}
	doer := &user_model.User{ID: 2, Name: "Doer"}

	ctx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})

	e := buildEvent(ctx, RecordParams{
		Action: audit_model.RepositoryMirrorPushAdd,
		Actor:  actorRef(doer),
		Scope:  ScopeFromRepository(r),
		Metadata: metaPairs(
			"mirror_id", m.ID,
			"remote_address", m.RemoteAddress,
		),
	})
	e.TimestampUnix = timeutil.TimeStamp(time.Time{}.Unix())

	sb := strings.Builder{}
	assert.NoError(t, WriteEventsAsJSON(&sb, []*audit_model.Event{e, e}))
	out := sb.String()
	assert.Equal(t, 2, strings.Count(out, "\n"))
	assert.Contains(t, out, `"action":"repository:mirror:push:add"`)
	assert.Contains(t, out, `"name":"Doer"`)
	assert.Contains(t, out, `"metadata"`)
	assert.Contains(t, out, `"remote_address":"git@example.com:repo.git"`)
	assert.Contains(t, out, `"ip_address":"127.0.0.1"`)
}
