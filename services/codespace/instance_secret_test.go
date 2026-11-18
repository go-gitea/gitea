// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"strconv"
	"strings"
	"testing"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSecretSources(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	require.NoError(t, CreateUserSecret(t.Context(), user, "TOKEN", "saved-value"))
	for _, tc := range []struct {
		name         string
		userID       int64
		savedAllowed bool
		inputs       []CreateSecretInput
		want         []RuntimeSecret
		wantErr      error
	}{
		{name: "selected saved value", userID: 2, savedAllowed: true, inputs: []CreateSecretInput{{Name: "token", Source: SecretSourceSaved}}, want: []RuntimeSecret{{Name: "TOKEN", Value: "saved-value"}}},
		{name: "explicit custom value", userID: 2, inputs: []CreateSecretInput{{Name: "TOKEN", Source: SecretSourceCustom, Value: " first\nsecond\n"}}, want: []RuntimeSecret{{Name: "TOKEN", Value: " first\nsecond\n"}}},
		{name: "blank and omitted values", userID: 2, savedAllowed: true, inputs: []CreateSecretInput{{Name: "TOKEN", Source: SecretSourceCustom}, {Name: "OTHER", Source: SecretSourceNone, Value: "ignored"}}},
		{name: "unselected saved value", userID: 2, savedAllowed: true},
		{name: "restricted source", userID: 2, inputs: []CreateSecretInput{{Name: "TOKEN", Source: SecretSourceSaved}}, wantErr: ErrCreateSecretUnavailable},
		{name: "secret owner", userID: 4, savedAllowed: true, inputs: []CreateSecretInput{{Name: "TOKEN", Source: SecretSourceSaved}}, wantErr: ErrCreateSecretUnavailable},
		{name: "duplicate name", userID: 2, inputs: []CreateSecretInput{{Name: "TOKEN", Source: SecretSourceNone}, {Name: "token", Source: SecretSourceCustom, Value: "value"}}, wantErr: ErrUserSecretNameConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := encryptCreateSecrets(t.Context(), tc.userID, tc.savedAllowed, tc.inputs)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			values, err := decryptCodespaceSecrets(&codespace_model.Codespace{SecretsEncrypted: ciphertext})
			require.NoError(t, err)
			assert.Equal(t, tc.want, values)
		})
	}
}

func TestCreateSecretSelectionSize(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	value := strings.Repeat("x", userSecretValueSizeLimit)
	var inputs []CreateSecretInput
	for i := range instanceSecretTotalSizeLimit/userSecretValueSizeLimit + 1 {
		name := "SECRET_" + strconv.Itoa(i)
		require.NoError(t, CreateUserSecret(t.Context(), user, name, value))
		inputs = append(inputs, CreateSecretInput{Name: name, Source: SecretSourceSaved})
	}
	_, err := encryptCreateSecrets(t.Context(), user.ID, true, inputs[:len(inputs)-1])
	require.NoError(t, err)
	_, err = encryptCreateSecrets(t.Context(), user.ID, true, inputs)
	require.ErrorIs(t, err, ErrCreateSecretSizeLimit)
	inputs[len(inputs)-1] = CreateSecretInput{Name: "CUSTOM", Source: SecretSourceCustom, Value: value}
	_, err = encryptCreateSecrets(t.Context(), user.ID, true, inputs)
	require.ErrorIs(t, err, ErrCreateSecretSizeLimit)
}
