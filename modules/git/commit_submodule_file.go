// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

var scpSyntax = regexp.MustCompile(`^([a-zA-Z0-9_]+@)?([a-zA-Z0-9._-]+):(.*)$`)

// CommitSubModuleFile represents a file with submodule type.
type CommitSubModuleFile struct {
	refURL string
	refID  string
}

// NewCommitSubModuleFile create a new submodule file
func NewCommitSubModuleFile(refURL, refID string) *CommitSubModuleFile {
	return &CommitSubModuleFile{
		refURL: refURL,
		refID:  refID,
	}
}

func getRefURL(refURL, repoFullName string) string {
	if refURL == "" {
		return ""
	}

	// FIXME: use a more generic way to handle the domain and subpath
	urlPrefix := setting.AppURL
	sshDomain := setting.SSH.Domain

	refURI := strings.TrimSuffix(refURL, ".git")

	prefixURL, _ := url.Parse(urlPrefix)
	urlPrefixHostname, _, err := net.SplitHostPort(prefixURL.Host)
	if err != nil {
		urlPrefixHostname = prefixURL.Host
	}

	urlPrefix = strings.TrimSuffix(urlPrefix, "/")

	// FIXME: Need to consider branch - which will require changes in modules/git/commit.go:GetSubModules
	// Relative url prefix check (according to git submodule documentation)
	if strings.HasPrefix(refURI, "./") || strings.HasPrefix(refURI, "../") {
		return urlPrefix + path.Clean(path.Join("/", repoFullName, refURI))
	}

	if !strings.Contains(refURI, "://") {
		// scp style syntax which contains *no* port number after the : (and is not parsed by net/url)
		// ex: git@try.gitea.io:go-gitea/gitea
		match := scpSyntax.FindAllStringSubmatch(refURI, -1)
		if len(match) > 0 {
			m := match[0]
			refHostname := m[2]
			pth := m[3]

			if !strings.HasPrefix(pth, "/") {
				pth = "/" + pth
			}

			if urlPrefixHostname == refHostname || refHostname == sshDomain {
				return urlPrefix + path.Clean(path.Join("/", pth))
			}
			return "http://" + refHostname + pth
		}
	}

	ref, err := url.Parse(refURI)
	if err != nil {
		return ""
	}

	refHostname, _, err := net.SplitHostPort(ref.Host)
	if err != nil {
		refHostname = ref.Host
	}

	supportedSchemes := []string{"http", "https", "git", "ssh", "git+ssh"}

	for _, scheme := range supportedSchemes {
		if ref.Scheme == scheme {
			if ref.Scheme == "http" || ref.Scheme == "https" {
				if len(ref.User.Username()) > 0 {
					return ref.Scheme + "://" + fmt.Sprintf("%v", ref.User) + "@" + ref.Host + ref.Path
				}
				return ref.Scheme + "://" + ref.Host + ref.Path
			} else if urlPrefixHostname == refHostname || refHostname == sshDomain {
				return urlPrefix + path.Clean(path.Join("/", ref.Path))
			}
			return "http://" + refHostname + ref.Path
		}
	}

	return ""
}

// RefURL guesses and returns reference URL.
func (sf *CommitSubModuleFile) RefURL(repoFullName string) string {
	return getRefURL(sf.refURL, repoFullName)
}

// RefID returns reference ID.
func (sf *CommitSubModuleFile) RefID() string {
	return sf.refID
}
