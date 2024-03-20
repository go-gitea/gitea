// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// This file contains functions for creating authorized_principals files
//
// There is a dependence on the database within RewriteAllPrincipalKeys & RegeneratePrincipalKeys
// The sshOpLocker is used from ssh_key_authorized_keys.go

const (
	authorizedPrincipalsFile = "authorized_principals"
	tplCommentPrefix         = `# gitea public key`
)

// RewriteAllPrincipalKeys removes any authorized principal and rewrite all keys from database again.
// Note: db.GetEngine(ctx).Iterate does not get latest data after insert/delete, so we have to call this function
// outside any session scope independently.
func RewriteAllPrincipalKeys(ctx context.Context) error {
	// Don't rewrite key if internal server
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedPrincipalsFile {
		return nil
	}

	return asymkey_model.WithSSHOpLocker(func() error {
		return rewriteAllPrincipalKeys(ctx)
	})
}

func rewriteAllPrincipalKeys(ctx context.Context) error {
	if setting.SSH.RootPath != "" {
		// First of ensure that the RootPath is present, and if not make it with 0700 permissions
		// This of course doesn't guarantee that this is the right directory for authorized_keys
		// but at least if it's supposed to be this directory and it doesn't exist and we're the
		// right user it will at least be created properly.
		err := os.MkdirAll(setting.SSH.RootPath, 0o700)
		if err != nil {
			log.Error("Unable to MkdirAll(%s): %v", setting.SSH.RootPath, err)
			return err
		}
	}

	fPath := filepath.Join(setting.SSH.RootPath, authorizedPrincipalsFile)
	tmpPath := fPath + ".tmp"
	t, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		t.Close()
		os.Remove(tmpPath)
	}()

	if setting.SSH.AuthorizedPrincipalsBackup {
		isExist, err := util.IsExist(fPath)
		if err != nil {
			log.Error("Unable to check if %s exists. Error: %v", fPath, err)
			return err
		}
		if isExist {
			bakPath := fmt.Sprintf("%s_%d.gitea_bak", fPath, time.Now().Unix())
			if err = util.CopyFile(fPath, bakPath); err != nil {
				return err
			}
		}
	}

	if err := regeneratePrincipalKeys(ctx, t); err != nil {
		return err
	}

	t.Close()
	return util.Rename(tmpPath, fPath)
}

func regeneratePrincipalKeys(ctx context.Context, t io.StringWriter) error {
	if err := db.GetEngine(ctx).Where("type = ?", asymkey_model.KeyTypePrincipal).Iterate(new(asymkey_model.PublicKey), func(idx int, bean any) (err error) {
		_, err = t.WriteString((bean.(*asymkey_model.PublicKey)).AuthorizedString())
		return err
	}); err != nil {
		return err
	}

	fPath := filepath.Join(setting.SSH.RootPath, authorizedPrincipalsFile)
	isExist, err := util.IsExist(fPath)
	if err != nil {
		log.Error("Unable to check if %s exists. Error: %v", fPath, err)
		return err
	}
	if isExist {
		f, err := os.Open(fPath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, tplCommentPrefix) {
				scanner.Scan()
				continue
			}
			_, err = t.WriteString(line + "\n")
			if err != nil {
				f.Close()
				return err
			}
		}
		err = scanner.Err()
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		f.Close()
	}
	return nil
}
