// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package repository

import (
	"os"
	"os/exec"
	"path/filepath"

	"gitea.dev/modules/util"

	"golang.org/x/sys/windows"
)

// defaultSSHCommand falls back to the OpenSSH client bundled with Windows, which isn't always on PATH
func defaultSSHCommand() string {
	if _, err := exec.LookPath("ssh"); err == nil {
		return "ssh"
	}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		bundled := filepath.Join(systemRoot, "System32", "OpenSSH", "ssh.exe")
		if exist, _ := util.IsExist(bundled); exist {
			return bundled
		}
	}
	return "ssh"
}

// restrictToCurrentUser replaces the inherited ACL of dir and its future files, Win32-OpenSSH rejects keys other accounts can access
func restrictToCurrentUser(dir string) error {
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(tokenUser.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
