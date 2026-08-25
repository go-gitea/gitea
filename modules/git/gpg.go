// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/process"
	"gitea.dev/modules/util"
)

type CommitSignSettings struct {
	Sign  bool
	Email string
	Name  string

	Format string // default to GPG
	KeyID  string // GPG key id or SSH key file

	cachedPublicKeyContent atomic.Pointer[string]
}

func (css *CommitSignSettings) PublicKeyContent() (string, error) {
	cached := css.cachedPublicKeyContent.Load()
	if cached != nil {
		return *cached, nil
	}

	if css.Format == SigningKeyFormatSSH {
		content, err := os.ReadFile(css.KeyID)
		if err != nil {
			return "", fmt.Errorf("unable to read SSH public key file: %s, %w", css.KeyID, err)
		}
		s := string(content)
		css.cachedPublicKeyContent.Store(&s)
		return s, nil
	}

	content, stderr, err := process.GetManager().Exec("gpg -a --export", "gpg", "-a", "--export", css.KeyID)
	if err != nil {
		return "", fmt.Errorf("unable to get default signing key: %s, %s, %w", css.KeyID, stderr, err)
	}
	css.cachedPublicKeyContent.Store(&content)
	return content, nil
}

var GlobalCommitSignSettings = util.OnceValue[*CommitSignSettings]{
	Func: func() *CommitSignSettings {
		ctx := context.Background()
		css := &CommitSignSettings{}

		// all errors are ignored because the keys might not exist
		// "--type=bool" resolves a valueless "commit.gpgsign" to true
		value, _, _ := gitcmd.NewCommand("config", "--global", "--default", "false", "--type=bool", "--get", "commit.gpgsign").RunStdString(ctx)
		css.Sign = strings.TrimSpace(value) == "true"

		signingKey, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.signingkey").RunStdString(ctx)
		css.KeyID = strings.TrimSpace(signingKey)
		css.Sign = css.Sign && css.KeyID != ""

		format, _, _ := gitcmd.NewCommand("config", "--global", "--default", SigningKeyFormatOpenPGP, "--get", "gpg.format").RunStdString(ctx)
		css.Format = strings.TrimSpace(format)

		defaultEmail, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.email").RunStdString(ctx)
		css.Email = strings.TrimSpace(defaultEmail)

		defaultName, _, _ := gitcmd.NewCommand("config", "--global", "--get", "user.name").RunStdString(ctx)
		css.Name = strings.TrimSpace(defaultName)
		return css
	},
}
