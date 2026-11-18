// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/json"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

const (
	SecretSourceSaved  = "saved"
	SecretSourceCustom = "custom"
	SecretSourceNone   = "none"

	// MaxCreateSecrets limits both configuration recommendations and submitted selections.
	MaxCreateSecrets             = 100
	instanceSecretTotalSizeLimit = 512 * 1024
)

var (
	ErrCreateSecretUnavailable = errors.New("selected saved secret is unavailable")
	ErrCreateSecretSizeLimit   = errors.New("codespace instance secret size limit exceeded")
)

// CreateSecretInput describes one explicitly selected source, without changing the saved secret.
type CreateSecretInput struct {
	Name   string
	Source string
	Value  string
}

// RuntimeSecret is one environment variable from the instance's creation snapshot.
type RuntimeSecret struct {
	Name  string
	Value string
}

func encryptCreateSecrets(ctx context.Context, userID int64, savedAllowed bool, inputs []CreateSecretInput) (string, error) {
	if len(inputs) > MaxCreateSecrets {
		return "", ErrUserSecretCountLimit
	}
	seen := make(map[string]struct{}, len(inputs))
	values := make(map[string]string, len(inputs))
	var savedNames []string
	for _, input := range inputs {
		name := strings.ToUpper(strings.TrimSpace(input.Name))
		if err := validateUserSecretName(name); err != nil {
			return "", err
		}
		if _, exists := seen[name]; exists {
			return "", ErrUserSecretNameConflict
		}
		seen[name] = struct{}{}
		switch input.Source {
		case SecretSourceNone:
		case SecretSourceCustom:
			if input.Value != "" {
				if err := validateUserSecretValue(input.Value); err != nil {
					return "", err
				}
				values[name] = input.Value
			}
		case SecretSourceSaved:
			if !savedAllowed {
				return "", ErrCreateSecretUnavailable
			}
			savedNames = append(savedNames, name)
		default:
			return "", util.NewInvalidArgumentErrorf("invalid secret source")
		}
	}
	if len(savedNames) > 0 {
		var saved []*codespace_model.UserSecret
		if err := db.GetEngine(ctx).Where("user_id = ?", userID).
			In("name", savedNames).Cols("name", "data_encrypted").Find(&saved); err != nil {
			return "", err
		}
		if len(saved) != len(savedNames) {
			return "", ErrCreateSecretUnavailable
		}
		for _, secret := range saved {
			value, err := secret_module.DecryptSecret(setting.SecretKey, secret.DataEncrypted)
			if err != nil {
				return "", err
			}
			if err := validateUserSecretValue(value); err != nil {
				return "", err
			}
			values[secret.Name] = value
		}
	}
	if len(values) == 0 {
		return "", nil
	}
	totalSize := 0
	for _, value := range values {
		totalSize += len(value)
	}
	if totalSize > instanceSecretTotalSizeLimit {
		return "", ErrCreateSecretSizeLimit
	}
	content, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return secret_module.EncryptSecret(setting.SecretKey, string(content))
}

func decryptCodespaceSecrets(codespace *codespace_model.Codespace) ([]RuntimeSecret, error) {
	if codespace.SecretsEncrypted == "" {
		return nil, nil
	}
	content, err := secret_module.DecryptSecret(setting.SecretKey, codespace.SecretsEncrypted)
	if err != nil {
		return nil, err
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(content), &values); err != nil {
		return nil, err
	}
	secrets := make([]RuntimeSecret, 0, len(values))
	for _, name := range slices.Sorted(maps.Keys(values)) {
		secrets = append(secrets, RuntimeSecret{Name: name, Value: values[name]})
	}
	return secrets, nil
}
