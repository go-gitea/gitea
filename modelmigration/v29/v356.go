// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

	"xorm.io/builder"
)

func reencryptColumn(x base.EngineMigration, table, column string, cond builder.Cond, reencrypt func(string) (string, error)) error {
	var lastID int64
	for {
		rows, err := x.Table(table).Cols("id", column).
			Where(builder.Gt{"id": lastID}.And(builder.Neq{column: ""}, cond)).
			OrderBy("id").Limit(100).QueryString()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if lastID, err = strconv.ParseInt(row["id"], 10, 64); err != nil {
				return err
			}
			value, err := reencrypt(row[column])
			if err != nil {
				log.Warn("Unable to re-encrypt %s.%s of id %d: %v", table, column, lastID, err)
			}
			if value == row[column] {
				continue
			}
			if _, err := x.Table(table).Where("id = ?", lastID).Update(map[string]any{column: value}); err != nil {
				return err
			}
		}
		if len(rows) < 100 {
			return nil
		}
	}
}

func reencryptValue(value string, decrypt func(key, encrypted string) (string, error)) (string, error) {
	if strings.HasPrefix(value, "v2:") {
		return value, nil
	}
	plaintext, err := decrypt(setting.SecretKey, value)
	if err != nil {
		return value, err
	}
	encrypted, err := secret.EncryptSecret(setting.SecretKey, plaintext)
	if err != nil {
		return value, err
	}
	return encrypted, nil
}

func reencryptHex(value string) (string, error) {
	return reencryptValue(value, secret.DecryptSecret)
}

func reencryptTOTP(value string) (string, error) {
	return reencryptValue(value, func(key, encrypted string) (string, error) {
		ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
		if err != nil {
			return "", err
		}
		md5Key := md5.Sum([]byte(key))
		plaintext, err := secret.AesDecrypt(md5Key[:], ciphertext)
		return string(plaintext), err
	})
}

func reencryptJSONFields(value string, fields ...string) (string, error) {
	var obj map[string]json.Value
	if err := json.UnmarshalHandleDoubleEncode([]byte(value), &obj); err != nil {
		return value, err
	}
	var errs []error
	changed := false
	for _, field := range fields {
		var encrypted string
		if obj[field] == nil || json.Unmarshal(obj[field], &encrypted) != nil || encrypted == "" {
			continue
		}
		reencrypted, err := reencryptHex(encrypted)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", field, err))
		} else if reencrypted != encrypted {
			if obj[field], err = json.Marshal(reencrypted); err != nil {
				return value, err
			}
			changed = true
		}
	}
	if !changed {
		return value, errors.Join(errs...)
	}
	bs, err := json.Marshal(obj)
	if err != nil {
		return value, err
	}
	return string(bs), errors.Join(errs...)
}

func ReencryptSecrets(_ context.Context, x base.EngineMigration) error {
	if err := reencryptColumn(x, "two_factor", "secret", nil, reencryptTOTP); err != nil {
		return err
	}
	if err := reencryptColumn(x, "webhook", "header_authorization_encrypted", nil, reencryptHex); err != nil {
		return err
	}
	if err := reencryptColumn(x, "secret", "data", nil, reencryptHex); err != nil {
		return err
	}
	if err := reencryptColumn(x, "login_source", "cfg", builder.In("type", 2, 5), func(value string) (string, error) {
		return reencryptJSONFields(value, "BindPasswordEncrypt")
	}); err != nil {
		return err
	}
	return reencryptColumn(x, "task", "payload_content", builder.Like{"payload_content", "_encrypted"}, func(value string) (string, error) {
		return reencryptJSONFields(value, "clone_addr_encrypted", "auth_password_encrypted", "auth_token_encrypted", "aws_secret_access_key_encrypted")
	})
}
