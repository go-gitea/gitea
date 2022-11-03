// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_19 //nolint

import (
	"fmt"

	"code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"xorm.io/builder"
	"xorm.io/xorm"
)

func batchProcess[T any](x *xorm.Engine, buf []T, query func(limit, start int) *xorm.Session, process func(*xorm.Session, T) error) error {
	size := cap(buf)
	start := 0
	for {
		err := query(size, start).Find(&buf)
		if err != nil {
			return err
		}
		if len(buf) == 0 {
			return nil
		}

		err = func() error {
			sess := x.NewSession()
			defer sess.Close()
			if err := sess.Begin(); err != nil {
				return fmt.Errorf("unable to allow start session. Error: %w", err)
			}
			for _, record := range buf {
				if err := process(sess, record); err != nil {
					return err
				}
			}
			return sess.Commit()
		}()
		if err != nil {
			return err
		}

		if len(buf) < size {
			return nil
		}
		start += size
		buf = buf[:0]
	}
}

func AddHeaderAuthorizationEncryptedColWebhook(x *xorm.Engine) error {
	// Add the column to the table
	type Webhook struct {
		ID   int64            `xorm:"pk autoincr"`
		Type webhook.HookType `xorm:"VARCHAR(16) 'type'"`
		Meta string           `xorm:"TEXT"` // store hook-specific attributes

		// HeaderAuthorizationEncrypted should be accessed using HeaderAuthorization() and SetHeaderAuthorization()
		HeaderAuthorizationEncrypted string `xorm:"TEXT"`
	}
	err := x.Sync(new(Webhook))
	if err != nil {
		return err
	}

	// Migrate the matrix webhooks

	type MatrixMeta struct {
		HomeserverURL string `json:"homeserver_url"`
		Room          string `json:"room_id"`
		MessageType   int    `json:"message_type"`
	}
	type MatrixMetaWithAccessToken struct {
		MatrixMeta
		AccessToken string `json:"access_token"`
	}

	err = batchProcess(x,
		make([]*Webhook, 0, 50),
		func(limit, start int) *xorm.Session {
			return x.Where("type=?", "matrix").OrderBy("id").Limit(limit, start)
		},
		func(sess *xorm.Session, hook *Webhook) error {
			// retrieve token from meta
			var withToken MatrixMetaWithAccessToken
			err := json.Unmarshal([]byte(hook.Meta), &withToken)
			if err != nil {
				return fmt.Errorf("unable to unmarshal matrix meta for webhook[id=%d]: %w", hook.ID, err)
			}
			if withToken.AccessToken == "" {
				return nil
			}

			// encrypt token
			authorization := "Bearer " + withToken.AccessToken
			hook.HeaderAuthorizationEncrypted, err = secret.EncryptSecret(setting.SecretKey, authorization)
			if err != nil {
				return fmt.Errorf("unable to encrypt access token for webhook[id=%d]: %w", hook.ID, err)
			}

			// remove token from meta
			withoutToken, err := json.Marshal(withToken.MatrixMeta)
			if err != nil {
				return fmt.Errorf("unable to marshal matrix meta for webhook[id=%d]: %w", hook.ID, err)
			}
			hook.Meta = string(withoutToken)

			// save in database
			count, err := sess.ID(hook.ID).Cols("meta", "header_authorization_encrypted").Update(hook)
			if count != 1 || err != nil {
				return fmt.Errorf("unable to update header_authorization_encrypted for webhook[id=%d]: %d,%w", hook.ID, count, err)
			}
			return nil
		})
	if err != nil {
		return err
	}

	// Remove access_token from HookTask

	type HookTask struct {
		ID             int64 `xorm:"pk autoincr"`
		HookID         int64
		PayloadContent string `xorm:"LONGTEXT"`
	}

	type MatrixPayloadSafe struct {
		Body          string               `json:"body"`
		MsgType       string               `json:"msgtype"`
		Format        string               `json:"format"`
		FormattedBody string               `json:"formatted_body"`
		Commits       []*api.PayloadCommit `json:"io.gitea.commits,omitempty"`
	}
	type MatrixPayloadUnsafe struct {
		MatrixPayloadSafe
		AccessToken string `json:"access_token"`
	}

	err = batchProcess(x,
		make([]*HookTask, 0, 50),
		func(limit, start int) *xorm.Session {
			return x.Where(builder.And(
				builder.In("hook_id", builder.Select("id").From("webhook").Where(builder.Eq{"type": "matrix"})),
				builder.Like{"payload_content", "access_token"},
			)).OrderBy("id").Limit(limit, 0) // ignore the provided "start", since other payload were already converted and don't contain 'payload_content' anymore
		},
		func(sess *xorm.Session, hookTask *HookTask) error {
			// retrieve token from payload_content
			var withToken MatrixPayloadUnsafe
			err := json.Unmarshal([]byte(hookTask.PayloadContent), &withToken)
			if err != nil {
				return fmt.Errorf("unable to unmarshal payload_content for hook_task[id=%d]: %w", hookTask.ID, err)
			}
			if withToken.AccessToken == "" {
				return nil
			}

			// remove token from payload_content
			withoutToken, err := json.Marshal(withToken.MatrixPayloadSafe)
			if err != nil {
				return fmt.Errorf("unable to marshal payload_content for hook_task[id=%d]: %w", hookTask.ID, err)
			}
			hookTask.PayloadContent = string(withoutToken)

			// save in database
			count, err := sess.ID(hookTask.ID).Cols("payload_content").Update(hookTask)
			if count != 1 || err != nil {
				return fmt.Errorf("unable to update payload_content for hook_task[id=%d]: %d,%w", hookTask.ID, count, err)
			}
			return nil
		})
	if err != nil {
		return err
	}

	return nil
}
