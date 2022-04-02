// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// CustomPayload is a payload for a custom webhook
	CustomPayload struct {
		Form    map[string]interface{} `json:"form"`
		Payload api.Payloader          `json:"payload"`
	}

	// CustomMeta is the meta information for a custom webhook
	CustomMeta struct {
		DisplayName string
		Form        map[string]interface{}
		Secret      string
	}
)

// GetCustomHook returns custom metadata
func GetCustomHook(w *webhook_model.Webhook) *CustomMeta {
	s := &CustomMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetCustomHook(%d): %v", w.ID, err)
	}
	return s
}

func (c CustomPayload) JSONPayload() ([]byte, error) {
	return json.Marshal(c)
}

// GetCustomPayload converts a custom webhook into a CustomPayload
func GetCustomPayload(p api.Payloader, _ webhook_model.HookEventType, meta string) (api.Payloader, error) {
	s := new(CustomPayload)

	custom := &CustomMeta{}
	if err := json.Unmarshal([]byte(meta), &custom); err != nil {
		return s, errors.New("GetPackagistPayload meta json:" + err.Error())
	}
	s.Form = custom.Form
	s.Payload = p
	return s, nil
}
