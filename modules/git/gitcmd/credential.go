// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"net/url"
	"strings"

	"gitea.dev/modules/util"
)

// credentialHelper answers only for the address's origin, so requests redirected to another host get nothing
const credentialHelper = `!f() { while read -r l; do case $l in protocol=*) p=${l#*=} ;; host=*) h=${l#*=} ;; esac; done; if [ "$1" = get ] && [ "$p://$h" = "$GITEA_REMOTE_ORIGIN" ]; then printf 'username=%s\npassword=%s\n' "$GITEA_REMOTE_USERNAME" "$GITEA_REMOTE_PASSWORD"; fi; }; f`

func parseRemoteCredentials(addr string) *url.URL {
	u, err := url.Parse(addr)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User == nil {
		return nil
	}
	return u
}

// RemoteAddressWithoutCredentials removes the credentials from an HTTP(S) address, other addresses are returned as-is
func RemoteAddressWithoutCredentials(addr string) string {
	u := parseRemoteCredentials(addr)
	if u == nil {
		return addr
	}
	u.User = nil
	if u.Path == "" {
		u.Path = "/" // the origin rewrite of WithRemoteCredentials only matches with a path
	}
	return u.String()
}

// WithRemoteCredentials passes the credentials of an HTTP(S) address to git through the environment, keeping them out of the config and the arguments of git itself
func (c *Command) WithRemoteCredentials(addr string) *Command {
	u := parseRemoteCredentials(addr)
	if u == nil {
		return c
	}
	password, _ := u.User.Password()
	if strings.ContainsAny(u.User.Username()+password, "\r\n") || strings.Contains(u.Host, "=") {
		c.preErrors = append(c.preErrors, util.ErrorWrap(ErrBrokenCommand, "invalid remote credentials or host"))
		return c
	}
	c.AddConfig("credential.helper", "") // reset the configured helpers, so none stores the credentials
	if u.User.Username() == "" {
		// before 2.46 git only sends credentials without a username from the address, so the origin is rewritten to include them, git-remote-http gets them as an argument
		origin := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/"}).String()
		userinfo := strings.ReplaceAll(u.User.String(), "=", "%3D")
		param := "url." + strings.Replace(origin, "://", "://"+userinfo+"@", 1) + ".insteadOf=" + origin
		c.credentialConfig = "'" + strings.ReplaceAll(param, "'", `'\''`) + "'"
		return c
	}
	c.AddConfig("credential.helper", credentialHelper)
	c.credentialEnvs = []string{"GITEA_REMOTE_ORIGIN=" + u.Scheme + "://" + u.Host, "GITEA_REMOTE_USERNAME=" + u.User.Username(), "GITEA_REMOTE_PASSWORD=" + password}
	return c
}

// withCredentialEnvs appends the credential envs last, keeping the inherited config parameters, e.g. the egress proxy's
func (c *Command) withCredentialEnvs(env []string) []string {
	if c.credentialConfig == "" {
		return append(env, c.credentialEnvs...)
	}
	params := c.credentialConfig
	for _, kv := range env {
		if inherited, ok := strings.CutPrefix(kv, "GIT_CONFIG_PARAMETERS="); ok {
			params = strings.TrimSpace(inherited + " " + c.credentialConfig)
		}
	}
	return append(env, "GIT_CONFIG_PARAMETERS="+params)
}
