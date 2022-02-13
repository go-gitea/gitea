// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.package webhook

package webhook

import (
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// Custom contains metadata for the Custom WebHook
	CustomMeta struct {
		HostURL   string `json:"host_url"`
		AuthToken string `json:"auth_token,omitempty"`
	}
)

// GetCustomPayload returns the payload as-is
func GetCustomPayload(p api.Payloader, event webhook_model.HookEventType, meta string) (api.Payloader, error) {
	// TODO: add optional body on POST.
	return p, nil
}

// GetCustomHook returns Custom metadata
func GetCustomHook(w *webhook_model.Webhook) *CustomMeta {
	s := &CustomMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetCustomHook(%d): %v", w.ID, err)
	}
	return s
}
