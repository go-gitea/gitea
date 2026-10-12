// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitcmd

import (
	"net/url"
	"strconv"
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
		c.credentialConfig = [2]string{"url." + strings.Replace(origin, "://", "://"+u.User.String()+"@", 1) + ".insteadOf", origin}
		return c
	}
	c.AddConfig("credential.helper", credentialHelper)
	c.credentialEnvs = []string{"GITEA_REMOTE_ORIGIN=" + u.Scheme + "://" + u.Host, "GITEA_REMOTE_USERNAME=" + u.User.Username(), "GITEA_REMOTE_PASSWORD=" + password}
	return c
}

// ConfigEnvs returns the envs adding a command scope config on top of the GIT_CONFIG_COUNT ones in base, it keeps the config out of process listings
func ConfigEnvs(base []string, key, value string) []string {
	count := 0
	for _, kv := range base {
		if v, ok := strings.CutPrefix(kv, "GIT_CONFIG_COUNT="); ok {
			count, _ = strconv.Atoi(v)
		}
	}
	n := strconv.Itoa(count)
	return []string{"GIT_CONFIG_COUNT=" + strconv.Itoa(count+1), "GIT_CONFIG_KEY_" + n + "=" + key, "GIT_CONFIG_VALUE_" + n + "=" + value}
}

// withCredentialEnvs appends the credential envs last, keeping the inherited config envs, e.g. the egress proxy's
func (c *Command) withCredentialEnvs(env []string) []string {
	if c.credentialConfig[0] == "" {
		return append(env, c.credentialEnvs...)
	}
	return append(env, ConfigEnvs(env, c.credentialConfig[0], c.credentialConfig[1])...)
}
