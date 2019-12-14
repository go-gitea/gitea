// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var scpSyntax = regexp.MustCompile(`^([a-zA-Z0-9_]+@)?([a-zA-Z0-9._-]+):(.*)$`)

// SubModule submodule is a reference on git repository
type SubModule struct {
	Name string
	URL  string
}

// SubModuleFile represents a file with submodule type.
type SubModuleFile struct {
	*Commit

	refURL string
	refID  string
}

// NewSubModuleFile create a new submodule file
func NewSubModuleFile(c *Commit, refURL, refID string) *SubModuleFile {
	return &SubModuleFile{
		Commit: c,
		refURL: refURL,
		refID:  refID,
	}
}

func getRefURL(refURL, urlPrefix, parentPath string) string {
	if refURL == "" {
		return ""
	}

	refURI := strings.TrimSuffix(refURL, ".git")

	prefixURL, _ := url.Parse(urlPrefix)
	urlPrefixHostname, _, err := net.SplitHostPort(prefixURL.Host)
	if err != nil {
		urlPrefixHostname = prefixURL.Host
	}

	// Relative url prefix check (according to git submodule documentation)
	if strings.HasPrefix(refURI, "./") || strings.HasPrefix(refURI, "../") {
		// ...construct and return correct submodule url here...
		idx := strings.Index(parentPath, "/src/")
		if idx == -1 {
			return refURI
		}
		return strings.TrimSuffix(urlPrefix, "/") + parentPath[:idx] + "/" + refURI
	}

	if !strings.Contains(refURI, "://") {
		// scp style syntax which contains *no* port number after the : (and is not parsed by net/url)
		// ex: git@try.gitea.io:go-gitea/gitea
		match := scpSyntax.FindAllStringSubmatch(refURI, -1)
		if len(match) > 0 {

			m := match[0]
			refHostname := m[2]
			path := m[3]

			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}

			if urlPrefixHostname == refHostname {
				return prefixURL.Scheme + "://" + urlPrefixHostname + path
			}
			return "http://" + refHostname + path
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
			if urlPrefixHostname == refHostname {
				return prefixURL.Scheme + "://" + prefixURL.Host + ref.Path
			} else if ref.Scheme == "http" || ref.Scheme == "https" {
				if len(ref.User.Username()) > 0 {
					return ref.Scheme + "://" + fmt.Sprintf("%v", ref.User) + "@" + ref.Host + ref.Path
				}
				return ref.Scheme + "://" + ref.Host + ref.Path
			} else {
				return "http://" + refHostname + ref.Path
			}
		}
	}

	return ""
}

// RefURL guesses and returns reference URL.
func (sf *SubModuleFile) RefURL(urlPrefix string, parentPath string) string {
	return getRefURL(sf.refURL, urlPrefix, parentPath)
}

// RefID returns reference ID.
func (sf *SubModuleFile) RefID() string {
	return sf.refID
}
