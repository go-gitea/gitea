// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const tplCommentPrefix = `# gitea public key`

func checkAuthorizedKeys(ctx context.Context, logger log.Logger, autofix bool) error {
	if setting.SSH.StartBuiltinServer || !setting.SSH.CreateAuthorizedKeysFile {
		return nil
	}

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.Open(fPath)
	if err != nil {
		if !autofix {
			logger.Critical("Unable to open authorized_keys file. ERROR: %v", err)
			return fmt.Errorf("Unable to open authorized_keys file. ERROR: %w", err)
		}
		logger.Warn("Unable to open authorized_keys. (ERROR: %v). Attempting to rewrite...", err)
		if err = asymkey_model.RewriteAllPublicKeys(ctx); err != nil {
			logger.Critical("Unable to rewrite authorized_keys file. ERROR: %v", err)
			return fmt.Errorf("Unable to rewrite authorized_keys file. ERROR: %w", err)
		}
	}
	defer f.Close()

	linesInAuthorizedKeys := make(container.Set[string])

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, tplCommentPrefix) {
			continue
		}
		linesInAuthorizedKeys.Add(line)
	}
	f.Close()

	// now we regenerate and check if there are any lines missing
	regenerated := &bytes.Buffer{}
	if err := asymkey_model.RegeneratePublicKeys(ctx, regenerated); err != nil {
		logger.Critical("Unable to regenerate authorized_keys file. ERROR: %v", err)
		return fmt.Errorf("Unable to regenerate authorized_keys file. ERROR: %w", err)
	}
	scanner = bufio.NewScanner(regenerated)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, tplCommentPrefix) {
			continue
		}
		if linesInAuthorizedKeys.Contains(line) {
			continue
		}
		if !autofix {
			logger.Critical(
				"authorized_keys file %q is out of date.\nRegenerate it with:\n\t\"%s\"\nor\n\t\"%s\"",
				fPath,
				"gitea admin regenerate keys",
				"gitea doctor --run authorized-keys --fix")
			return fmt.Errorf(`authorized_keys is out of date and should be regenerated with "gitea admin regenerate keys" or "gitea doctor --run authorized-keys --fix"`)
		}
		logger.Warn("authorized_keys is out of date. Attempting rewrite...")
		err = asymkey_model.RewriteAllPublicKeys(ctx)
		if err != nil {
			logger.Critical("Unable to rewrite authorized_keys file. ERROR: %v", err)
			return fmt.Errorf("Unable to rewrite authorized_keys file. ERROR: %w", err)
		}
	}
	return nil
}

func init() {
	Register(&Check{
		Title:     "Check if OpenSSH authorized_keys file is up-to-date",
		Name:      "authorized-keys",
		IsDefault: true,
		Run:       checkAuthorizedKeys,
		Priority:  4,
	})
}
