// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// GetRemoteAddress returns remote url of git repository in the repoPath with special remote name
func GetRemoteAddress(ctx context.Context, repoPath, remoteName string) (string, error) {
	var cmd *Command
	if DefaultFeatures().CheckVersionAtLeast("2.7") {
		cmd = NewCommand("remote", "get-url").AddDynamicArguments(remoteName)
	} else {
		cmd = NewCommand("config", "--get").AddDynamicArguments("remote." + remoteName + ".url")
	}

	result, _, err := cmd.RunStdString(ctx, &RunOpts{Dir: repoPath})
	if err != nil {
		return "", err
	}

	if len(result) > 0 {
		result = result[:len(result)-1]
	}
	return result, nil
}

// ErrInvalidCloneAddr represents a "InvalidCloneAddr" kind of error.
type ErrInvalidCloneAddr struct {
	Host               string
	IsURLError         bool
	IsInvalidPath      bool
	IsProtocolInvalid  bool
	IsPermissionDenied bool
	LocalPath          bool
}

// IsErrInvalidCloneAddr checks if an error is a ErrInvalidCloneAddr.
func IsErrInvalidCloneAddr(err error) bool {
	_, ok := err.(*ErrInvalidCloneAddr)
	return ok
}

func (err *ErrInvalidCloneAddr) Error() string {
	if err.IsInvalidPath {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided path is invalid", err.Host)
	}
	if err.IsProtocolInvalid {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided url protocol is not allowed", err.Host)
	}
	if err.IsPermissionDenied {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed.", err.Host)
	}
	if err.IsURLError {
		return fmt.Sprintf("migration/cloning from '%s' is not allowed: the provided url is invalid", err.Host)
	}

	return fmt.Sprintf("migration/cloning from '%s' is not allowed", err.Host)
}

func (err *ErrInvalidCloneAddr) Unwrap() error {
	return util.ErrInvalidArgument
}

// IsRemoteNotExistError checks the prefix of the error message to see whether a remote does not exist.
func IsRemoteNotExistError(err error) bool {
	// see: https://github.com/go-gitea/gitea/issues/32889#issuecomment-2571848216
	// Should not add space in the end, sometimes git will add a `:`
	prefix1 := "exit status 128 - fatal: No such remote" // git < 2.30
	prefix2 := "exit status 2 - error: No such remote"   // git >= 2.30
	return strings.HasPrefix(err.Error(), prefix1) || strings.HasPrefix(err.Error(), prefix2)
}

// normalizeSSHURL converts SSH-SCP format URLs to standard ssh:// format for security
func normalizeSSHURL(remoteAddr string) (string, error) {
	if strings.HasPrefix(remoteAddr, "ssh://") {
		return remoteAddr, nil
	}
	if strings.Contains(remoteAddr, "://") {
		return remoteAddr, errors.New("remoteAddr has a scheme")
	}
	if strings.Contains(remoteAddr, "\\") {
		return remoteAddr, errors.New("remoteAddr has Windows path slashes")
	}
	if strings.Contains(remoteAddr, ":/") {
		return remoteAddr, errors.New("remoteAddr could be Windows drive with forward slash")
	}
	if remoteAddr != "" && (remoteAddr[0] == '/' || remoteAddr[0] == '\\') {
		return remoteAddr, errors.New("remoteAddr is a local file path")
	}

	// Parse SSH-SCP format: [user@]host:path
	colonIndex := strings.Index(remoteAddr, ":")
	if colonIndex == -1 {
		return remoteAddr, errors.New("remoteAddr has no colon")
	}

	if colonIndex == 1 && len(remoteAddr) > 2 {
		return remoteAddr, errors.New("remoteAddr could be Windows drive letter check (C:, D:, etc.)")
	}

	hostPart := remoteAddr[:colonIndex]
	pathPart := remoteAddr[colonIndex+1:]

	if hostPart == "" || pathPart == "" {
		return remoteAddr, errors.New("remoteAddr has empty host or path")
	}

	var user, host string
	if atIndex := strings.LastIndex(hostPart, "@"); atIndex != -1 {
		user = hostPart[:atIndex+1] // Include the @
		host = hostPart[atIndex+1:]
	} else {
		user = "git@"
		host = hostPart
	}

	if host == "" {
		return remoteAddr, errors.New("Must have SSH host")
	}

	return fmt.Sprintf("ssh://%s%s/%s", user, host, pathPart), nil
}

// ParseRemoteAddr checks if given remote address is valid,
// and returns composed URL with needed username and password.
func ParseRemoteAddr(remoteAddr, authUsername, authPassword string) (string, error) {
	remoteAddr = strings.TrimSpace(remoteAddr)

	// First, try to normalize SSH-SCP format URLs to ssh:// format for security
	normalizedAddr, err := normalizeSSHURL(remoteAddr)
	if err == nil {
		remoteAddr = normalizedAddr
	}

	// Remote address can be HTTP/HTTPS/Git URL or SSH URL or local path.
	if strings.HasPrefix(remoteAddr, "http://") ||
		strings.HasPrefix(remoteAddr, "https://") ||
		strings.HasPrefix(remoteAddr, "git://") {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", &ErrInvalidCloneAddr{IsURLError: true, Host: remoteAddr}
		}
		if len(authUsername)+len(authPassword) > 0 {
			u.User = url.UserPassword(authUsername, authPassword)
		}
		remoteAddr = u.String()
	} else if strings.HasPrefix(remoteAddr, "ssh://") {
		// Handle ssh:// URLs (including normalized ones)
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", &ErrInvalidCloneAddr{IsURLError: true, Host: remoteAddr}
		}
		if len(authUsername)+len(authPassword) > 0 {
			// SSH URLs don't support username/password auth, only key-based auth
			return "", &ErrInvalidCloneAddr{IsURLError: true, Host: remoteAddr}
		}
		remoteAddr = u.String()
	}

	return remoteAddr, nil
}
