// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package models

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	activities_model "code.gitea.io/gitea/models/activities"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestUserHeatmap(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	token := getUserToken(t, adminUsername)

	fakeNow := time.Date(2011, 10, 20, 0, 0, 0, 0, time.Local)
	timeutil.Set(fakeNow)
	defer timeutil.Unset()

	urlStr := fmt.Sprintf("/api/v1/users/%s/heatmap?token=%s", normalUsername, token)
	req := NewRequest(t, "GET", urlStr)
	resp := MakeRequest(t, req, http.StatusOK)
	var heatmap []*activities_model.UserHeatmapData
	DecodeJSON(t, resp, &heatmap)
	var dummyheatmap []*activities_model.UserHeatmapData
	dummyheatmap = append(dummyheatmap, &activities_model.UserHeatmapData{Timestamp: 1603227600, Contributions: 1})

	assert.Equal(t, dummyheatmap, heatmap)
}
