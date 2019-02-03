// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "strings"

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

	url := strings.TrimSuffix(refURL, ".git")

	// git://xxx/user/repo
	if strings.HasPrefix(url, "git://") {
		return "http://" + strings.TrimPrefix(url, "git://")
	}

	// http[s]://xxx/user/repo
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}

	// Relative url prefix check (according to git submodule documentation)
	if strings.HasPrefix(url, "./") || strings.HasPrefix(url, "../") {
		// ...construct and return correct submodule url here...
		idx := strings.Index(parentPath, "/src/")
		if idx == -1 {
			return url
		}
		return strings.TrimSuffix(urlPrefix, "/") + parentPath[:idx] + "/" + url
	}

	// sysuser@xxx:user/repo
	i := strings.Index(url, "@")
	j := strings.LastIndex(url, ":")

	// Only process when i < j because git+ssh://git@git.forwardbias.in/npploader.git
	if i > -1 && j > -1 && i < j {
		// fix problem with reverse proxy works only with local server
		if strings.Contains(urlPrefix, url[i+1:j]) {
			return urlPrefix + url[j+1:]
		}
		if strings.HasPrefix(url, "ssh://") || strings.HasPrefix(url, "git+ssh://") {
			k := strings.Index(url[j+1:], "/")
			return "http://" + url[i+1:j] + "/" + url[j+1:][k+1:]
		}
		return "http://" + url[i+1:j] + "/" + url[j+1:]
	}

	return url
}

// RefURL guesses and returns reference URL.
func (sf *SubModuleFile) RefURL(urlPrefix string, parentPath string) string {
	return getRefURL(sf.refURL, urlPrefix, parentPath)
}

// RefID returns reference ID.
func (sf *SubModuleFile) RefID() string {
	return sf.refID
}
