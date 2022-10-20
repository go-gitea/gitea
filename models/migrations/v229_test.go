// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"testing"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func Test_addHeaderAuthorizationEncryptedColWebhook(t *testing.T) {
	// Create Webhook table
	type Webhook struct {
		ID                 int64 `xorm:"pk autoincr"`
		RepoID             int64 `xorm:"INDEX"` // An ID of 0 indicates either a default or system webhook
		OrgID              int64 `xorm:"INDEX"`
		IsSystemWebhook    bool
		URL                string `xorm:"url TEXT"`
		HTTPMethod         string `xorm:"http_method"`
		ContentType        webhook.HookContentType
		Secret             string `xorm:"TEXT"`
		Events             string `xorm:"TEXT"`
		*webhook.HookEvent `xorm:"-"`
		IsActive           bool               `xorm:"INDEX"`
		Type               webhook.HookType   `xorm:"VARCHAR(16) 'type'"`
		Meta               string             `xorm:"TEXT"` // store hook-specific attributes
		LastStatus         webhook.HookStatus // Last delivery status

		// HeaderAuthorizationEncrypted should be accessed using HeaderAuthorization() and SetHeaderAuthorization()
		HeaderAuthorizationEncrypted string `xorm:"TEXT"`

		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	type ExpectedWebhook struct {
		ID                  int64 `xorm:"pk autoincr"`
		Meta                string
		HeaderAuthorization string
	}

	type HookTask struct {
		ID              int64 `xorm:"pk autoincr"`
		RepoID          int64 `xorm:"INDEX"`
		HookID          int64
		UUID            string
		PayloadContent  string `xorm:"LONGTEXT"`
		EventType       string
		IsDelivered     bool
		Delivered       int64
		DeliveredString string `xorm:"-"`

		// History info.
		IsSucceed      bool
		RequestContent string `xorm:"LONGTEXT"`
		// RequestInfo     *HookRequest  `xorm:"-"`
		ResponseContent string `xorm:"LONGTEXT"`
		// ResponseInfo    *HookResponse `xorm:"-"`
	}

	// Prepare and load the testing database
	x, deferable := prepareTestEnv(t, 0, new(Webhook), new(ExpectedWebhook), new(HookTask))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := addHeaderAuthorizationEncryptedColWebhook(x); err != nil {
		assert.NoError(t, err)
		return
	}

	expected := []ExpectedWebhook{}
	if err := x.Table("expected_webhook").Asc("id").Find(&expected); !assert.NoError(t, err) {
		return
	}

	got := []Webhook{}
	if err := x.Table("webhook").Select("id, meta, header_authorization_encrypted").Asc("id").Find(&got); !assert.NoError(t, err) {
		return
	}

	for i, e := range expected {
		assert.Equal(t, e.Meta, got[i].Meta)

		if e.HeaderAuthorization == "" {
			assert.Equal(t, "", got[i].HeaderAuthorizationEncrypted)
		} else {
			cipherhex := got[i].HeaderAuthorizationEncrypted
			cleartext, err := secret.DecryptSecret(setting.SecretKey, cipherhex)
			assert.NoError(t, err)
			assert.Equal(t, e.HeaderAuthorization, cleartext)
		}
	}

	// ensure that no hook_task has some remaining "access_token"
	hookTasks := []HookTask{}
	if err := x.Table("hook_task").Select("id, payload_content").Asc("id").Find(&hookTasks); !assert.NoError(t, err) {
		return
	}
	for _, h := range hookTasks {
		var m map[string]interface{}
		err := json.Unmarshal([]byte(h.PayloadContent), &m)
		assert.NoError(t, err)
		assert.Nil(t, m["access_token"])
	}
}
