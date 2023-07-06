// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_19 //nolint

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	"github.com/stretchr/testify/assert"
)

func Test_AddHeaderAuthorizationEncryptedColWebhook(t *testing.T) {
	// Create Webhook table
	type Webhook struct {
		ID   int64                   `xorm:"pk autoincr"`
		Type webhook_module.HookType `xorm:"VARCHAR(16) 'type'"`
		Meta string                  `xorm:"TEXT"` // store hook-specific attributes

		// HeaderAuthorizationEncrypted should be accessed using HeaderAuthorization() and SetHeaderAuthorization()
		HeaderAuthorizationEncrypted string `xorm:"TEXT"`
	}

	type ExpectedWebhook struct {
		ID                  int64 `xorm:"pk autoincr"`
		Meta                string
		HeaderAuthorization string
	}

	type HookTask struct {
		ID             int64 `xorm:"pk autoincr"`
		HookID         int64
		PayloadContent string `xorm:"LONGTEXT"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(Webhook), new(ExpectedWebhook), new(HookTask))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	if err := AddHeaderAuthorizationEncryptedColWebhook(x); err != nil {
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
		var m map[string]any
		err := json.Unmarshal([]byte(h.PayloadContent), &m)
		assert.NoError(t, err)
		assert.Nil(t, m["access_token"])
	}
}
