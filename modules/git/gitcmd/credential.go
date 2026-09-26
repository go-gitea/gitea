// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"net/url"
	"strings"
)

// credentialHelper answers git's credential requests from the environment, so no secret is in the arguments
const credentialHelper = `!f() { if [ "$1" = get ]; then printf 'username=%s\npassword=%s\n' "$GITEA_REMOTE_USERNAME" "$GITEA_REMOTE_PASSWORD"; fi; }; f`

// WithRemoteCredentials provides the credentials embedded in an HTTP(S) address to git for that address's host.
// It does nothing if the address has no credentials.
func (c *Command) WithRemoteCredentials(addr string) *Command {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		return c
	}
	u, err := url.Parse(addr)
	if err != nil || u.User == nil {
		return c
	}
	password, _ := u.User.Password()
	// reset other helpers first, a URL-scoped helper keeps the credentials away from other hosts (e.g. redirections)
	c.AddConfig("credential.helper", "")
	c.AddConfig("credential."+u.Scheme+"://"+u.Host+".helper", credentialHelper)
	c.credentialEnvs = []string{"GITEA_REMOTE_USERNAME=" + u.User.Username(), "GITEA_REMOTE_PASSWORD=" + password}
	return c
}
