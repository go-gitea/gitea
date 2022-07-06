// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"testing"

	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/require"
)

type GiteaSecretModuleMock struct {
	DecryptCalled bool
	EncryptCalled bool
	SimulateError bool
}

func (m *GiteaSecretModuleMock) DecryptSecret(key, cipherhex string) (string, error) {
	m.DecryptCalled = true

	if m.SimulateError {
		return "", fmt.Errorf("Simulated error")
	}

	return cipherhex, nil
}

func (m *GiteaSecretModuleMock) EncryptSecret(key, str string) (string, error) {
	m.EncryptCalled = true

	if m.SimulateError {
		return "", fmt.Errorf("Simulated error")
	}

	return str, nil
}

func TestGetGiteaHook(t *testing.T) {
	t.Run("Legacy configuration", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: "",
		}

		m := GiteaSecretModuleMock{}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.NotNil(t, actual)
		require.IsType(t, &GiteaMeta{}, actual)
		require.False(t, actual.AuthHeaderEnabled)
		require.False(t, m.DecryptCalled, "Decrypt function unexpectedly called")
	})

	t.Run("Disabled auth headers", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: `{"auth_header_enabled": false}`,
		}

		m := GiteaSecretModuleMock{}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.NotNil(t, actual)
		require.IsType(t, &GiteaMeta{}, actual)
		require.False(t, actual.AuthHeaderEnabled)
		require.False(t, m.DecryptCalled, "Decrypt function unexpectedly called")
	})

	t.Run("Enabled auth headers", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: `{"auth_header_enabled": true, "auth_header": "{\"name\": \"X-Test-Authorization\", \"type\": \"basic\", \"username\": \"test-user\", \"password\":\"test-password\"}"}`,
		}

		m := GiteaSecretModuleMock{}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.NotNil(t, actual)
		require.IsType(t, &GiteaMeta{}, actual)
		require.True(t, actual.AuthHeaderEnabled)
		require.True(t, m.DecryptCalled, "Decrypt function was not called")

		require.Equal(t, "X-Test-Authorization", actual.AuthHeader.Name)
		require.Empty(t, actual.AuthHeaderData)
	})

	t.Run("Metadata parse error", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: `{"`,
		}

		m := GiteaSecretModuleMock{}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.Nil(t, actual)
		require.False(t, m.DecryptCalled, "Decrypt function unexpectedly called")
	})

	t.Run("AuthHeaderData parse error", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: `{"auth_header_enabled": true, "auth_header": "{\""}`,
		}

		m := GiteaSecretModuleMock{}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.Nil(t, actual)
		require.True(t, m.DecryptCalled, "Decrypt function was not called")
	})

	t.Run("Decryption error", func(t *testing.T) {
		s := &webhook_model.Webhook{
			Type: webhook_model.GITEA,
			Meta: `{"auth_header_enabled": true, "auth_header": "{\"name\": \"X-Test-Authorization\", \"type\": \"basic\", \"username\": \"test-user\", \"password\":\"test-password\"}"}`,
		}

		m := GiteaSecretModuleMock{SimulateError: true}

		actual := GetGiteaHook(s, m.DecryptSecret)

		require.Nil(t, actual)
		require.True(t, m.DecryptCalled, "Decrypt function was not called")
	})
}

func TestCreateGiteaHook(t *testing.T) {
	t.Run("Disabled auth headers", func(t *testing.T) {
		m := GiteaSecretModuleMock{}

		form := &forms.NewWebhookForm{
			AuthHeaderActive: false,
		}

		actual, err := CreateGiteaHook(form, m.EncryptSecret)
		expected := `{"auth_header_enabled":false}`

		require.Nil(t, err)
		require.Equal(t, expected, actual)
		require.False(t, m.EncryptCalled, "Encrypt function unexpectedly called")
	})

	t.Run("Enabled auth headers (basic auth)", func(t *testing.T) {
		m := GiteaSecretModuleMock{}

		form := &forms.NewWebhookForm{
			AuthHeaderActive:   true,
			AuthHeaderName:     "Authorization",
			AuthHeaderType:     webhook_model.BASICAUTH,
			AuthHeaderUsername: "test-user",
			AuthHeaderPassword: "test-password",
		}

		actual, err := CreateGiteaHook(form, m.EncryptSecret)
		expected := `{"auth_header_enabled":true,"auth_header":"{\"name\":\"Authorization\",\"type\":\"basic\",\"username\":\"test-user\",\"password\":\"test-password\"}"}`

		require.Nil(t, err)
		require.Equal(t, expected, actual)
		require.True(t, m.EncryptCalled, "Encrypt function was not called")
	})

	t.Run("Enabled auth headers (token auth)", func(t *testing.T) {
		m := GiteaSecretModuleMock{}

		form := &forms.NewWebhookForm{
			AuthHeaderActive: true,
			AuthHeaderName:   "Authorization",
			AuthHeaderType:   webhook_model.TOKENAUTH,
			AuthHeaderToken:  "test-token",
		}

		actual, err := CreateGiteaHook(form, m.EncryptSecret)
		expected := `{"auth_header_enabled":true,"auth_header":"{\"name\":\"Authorization\",\"type\":\"token\",\"token\":\"test-token\"}"}`

		require.Nil(t, err)
		require.Equal(t, expected, actual)
		require.True(t, m.EncryptCalled, "Encrypt function was not called")
	})

	t.Run("Encyption error", func(t *testing.T) {
		m := GiteaSecretModuleMock{SimulateError: true}

		form := &forms.NewWebhookForm{
			AuthHeaderActive: true,
			AuthHeaderName:   "Authorization",
			AuthHeaderType:   webhook_model.TOKENAUTH,
			AuthHeaderToken:  "test-token",
		}

		actual, err := CreateGiteaHook(form, m.EncryptSecret)

		require.NotNil(t, err)
		require.Errorf(t, err, "Simulated error")
		require.Empty(t, actual)
		require.True(t, m.EncryptCalled, "Encrypt function was not called")
	})
}
