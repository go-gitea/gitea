// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "strings"

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

func NewSubModuleFile(c *Commit, refURL, refID string) *SubModuleFile {
	return &SubModuleFile{
		Commit: c,
		refURL: refURL,
		refID:  refID,
	}
}

// RefURL guesses and returns reference URL.
func (sf *SubModuleFile) RefURL(urlPrefix string, parentPath string) string {
	if sf.refURL == "" {
		return ""
	}

	url := strings.TrimSuffix(sf.refURL, ".git")

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
		} else {
			return "http://" + url[i+1:j] + "/" + url[j+1:]
		}
	}

	return url
}

// RefID returns reference ID.
func (sf *SubModuleFile) RefID() string {
	return sf.refID
}
