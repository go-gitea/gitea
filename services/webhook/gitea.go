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
		Active   bool                         `json:"active"`
		Name     string                       `json:"name"`
		Type     webhook_model.AuthHeaderType `json:"type"`
		Username string                       `json:"username,omitempty"`
		Password string                       `json:"password,omitempty"`
		Token    string                       `json:"token,omitempty"`
	}

	// GiteaMeta contains the gitea webhook metadata
	GiteaMeta struct {
		AuthHeader GiteaAuthHeaderMeta `json:"authHeader"`
	}
)

// GetGiteaHook returns decrypted gitea metadata
func GetGiteaHook(w *webhook_model.Webhook) *GiteaMeta {
	meta, err := secret.DecryptSecret(setting.SecretKey, w.Meta)
	if err != nil {
		log.Error("webhook.GetGiteaHook(%d): %v", w.ID, err)
	}

	s := &GiteaMeta{}
	if err := json.Unmarshal([]byte(meta), s); err != nil {
		log.Error("webhook.GetGiteaHook(%d): %v", w.ID, err)
	}
	return s
}

// CreateGiteaHook creates an encrypted gitea metadata string. In case of errors,
// it returns an error message and the corresponding error. CreateGiteaHook ensures
// that only necessary data are stored in DB. Obsolete values are cleared.
func CreateGiteaHook(form *forms.NewWebhookForm) (meta string, errorMessage string, err error) {
	metaObject, err := json.Marshal(&GiteaMeta{
		AuthHeader: GiteaAuthHeaderMeta{
			Active:   form.AuthHeaderActive,
			Name:     form.AuthHeaderName,
			Type:     form.AuthHeaderType,
			Username: form.AuthHeaderUsername,
			Password: form.AuthHeaderPassword,
			Token:    form.AuthHeaderToken,
		},
	})
	if err != nil {
		return "", "Marshal", err
	}

	meta, err = secret.EncryptSecret(setting.SecretKey, string(metaObject))
	if err != nil {
		return "", "Encrypt", err
	}

	return meta, "", nil
}
