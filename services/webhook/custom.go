// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"os"
	"path/filepath"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
	cwebhook "code.gitea.io/gitea/modules/webhook"

	"github.com/google/go-jsonnet"
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
func GetCustomPayload(p api.Payloader, event webhook_model.HookEventType, w *webhook_model.Webhook) (api.Payloader, error) {
	s := new(CustomPayload)

	var custom CustomMeta
	if err := json.Unmarshal([]byte(w.Meta), &custom); err != nil {
		return s, fmt.Errorf("GetCustomPayload meta json: %v", err)
	}
	s.Form = custom.Form
	s.Payload = p

	payload, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("GetCustomPayload marshal json: %v", err)
	}

	webhook, ok := cwebhook.Webhooks[w.CustomID]
	if !ok {
		return nil, fmt.Errorf("GetCustomPayload no custom webhook %q", w.CustomID)
	}

	vm := jsonnet.MakeVM()
	vm.Importer(&jsonnet.MemoryImporter{
		Data: map[string]jsonnet.Contents{
			fmt.Sprintf("%s.libsonnet", event): jsonnet.MakeContents(string(payload)),
		},
	})

	filename := fmt.Sprintf("%s.jsonnet", event)
	snippet, err := os.ReadFile(filepath.Join(webhook.Path, filename))
	if err != nil {
		return nil, fmt.Errorf("GetCustomPayload read jsonnet: %v", err)
	}

	out, err := vm.EvaluateAnonymousSnippet(filename, string(snippet))
	return stringPayloader{out}, err
}

type stringPayloader struct {
	payload string
}

func (s stringPayloader) JSONPayload() ([]byte, error) {
	return []byte(s.payload), nil
}
