// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/forms"
)

type (
	// GiteaAuthHeaderMeta contains the authentication header metadata
	GiteaAuthHeaderMeta struct {
		Name     string                       `json:"name"`
		Type     webhook_model.AuthHeaderType `json:"type"`
		Username string                       `json:"username,omitempty"`
		Password string                       `json:"password,omitempty"`
		Token    string                       `json:"token,omitempty"`
	}

	// GiteaMeta contains the gitea webhook metadata
	GiteaMeta struct {
		AuthHeaderEnabled bool                 `json:"auth_header_enabled"`
		AuthHeaderData    string               `json:"auth_header,omitempty"`
		AuthHeader        GiteaAuthHeaderMeta `json:"-"`
	}
)

// GetGiteaHook returns decrypted gitea metadata
func GetGiteaHook(w *webhook_model.Webhook) *GiteaMeta {
	s := &GiteaMeta{}

	// Legacy webhook configuration has no stored metadata
	if w.Meta == "" {
		return s
	}

	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetGiteaHook(%d): %v", w.ID, err)
	}

	if !s.AuthHeaderEnabled {
		return s
	}

	headerData, err := secret.DecryptSecret(setting.SecretKey, s.AuthHeaderData)
	if err != nil {
		log.Error("webhook.GetGiteaHook(%d): %v", w.ID, err)
	}

	h := GiteaAuthHeaderMeta{}
	if err := json.Unmarshal([]byte(headerData), &h); err != nil {
		log.Error("webhook.GetGiteaHook(%d): %v", w.ID, err)
	}

	// Replace encrypted content with decrypted settings
	s.AuthHeaderData = ""
	s.AuthHeader = h

	return s
}

// CreateGiteaHook creates an gitea metadata string with encrypted auth header data,
// while it ensures to store the least necessary data in the database.
func CreateGiteaHook(form *forms.NewWebhookForm) (string, error) {
	metaObject := &GiteaMeta{
		AuthHeaderEnabled: form.AuthHeaderActive,
	}

	if form.AuthHeaderActive {
		headerMeta := GiteaAuthHeaderMeta{
			Name: form.AuthHeaderName,
			Type: form.AuthHeaderType,
		}

		switch form.AuthHeaderType {
		case webhook_model.BASICAUTH:
			headerMeta.Username = form.AuthHeaderUsername
			headerMeta.Password = form.AuthHeaderPassword
		case webhook_model.TOKENAUTH:
			headerMeta.Token = form.AuthHeaderToken
		}

		headerData, err := json.Marshal(headerMeta)
		if err != nil {
			return "", err
		}

		encryptedHeaderData, err := secret.EncryptSecret(setting.SecretKey, string(headerData))
		if err != nil {
			return "", err
		}

		metaObject.AuthHeaderData = encryptedHeaderData
	}

	meta, err := json.Marshal(metaObject)
	if err != nil {
		return "", err
	}

	return string(meta), nil
}
