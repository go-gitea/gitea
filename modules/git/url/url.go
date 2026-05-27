// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package url

import (
	"context"
	"fmt"
	"net"
	stdurl "net/url"
	"strings"

	"gitea.dev/modules/httplib"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

// ErrWrongURLFormat represents an error with wrong url format
type ErrWrongURLFormat struct {
	URL string
}

func (err ErrWrongURLFormat) Error() string {
	return fmt.Sprintf("git URL %s format is wrong", err.URL)
}

// GitURL represents a git URL
type GitURL struct {
	*stdurl.URL
	extraMark int // 0: standard URL with scheme, 1: scp short syntax (no scheme), 2: file path with no prefix
}

// String returns the URL's string
func (u *GitURL) String() string {
	switch u.extraMark {
	case 0:
		return u.URL.String()
	case 1:
		return fmt.Sprintf("%s@%s:%s", u.User.Username(), u.Host, u.Path)
	case 2:
		return u.Path
	default:
		return ""
	}
}

// ParseGitURL parse all kinds of git URL:
// * Full URL: http://git@host/path, http://git@host:port/path
// * SCP short syntax: git@host:/path
// * File path: /dir/repo/path
func ParseGitURL(remote string) (*GitURL, error) {
	if strings.Contains(remote, "://") {
		u, err := stdurl.Parse(remote)
		if err != nil {
			return nil, err
		}
		return &GitURL{URL: u}, nil
	} else if strings.Contains(remote, "@") && strings.Contains(remote, ":") {
		url := stdurl.URL{
			Scheme: "ssh",
		}
		squareBrackets := false
		lastIndex := -1
	FOR:
		for i := 0; i < len(remote); i++ {
			switch remote[i] {
			case '@':
				url.User = stdurl.User(remote[:i])
				lastIndex = i + 1
			case ':':
				if !squareBrackets {
					url.Host = strings.ReplaceAll(remote[lastIndex:i], "%25", "%")
					if len(remote) <= i+1 {
						return nil, ErrWrongURLFormat{URL: remote}
					}
					url.Path = remote[i+1:]
					break FOR
				}
			case '[':
				squareBrackets = true
			case ']':
				squareBrackets = false
			}
		}
		return &GitURL{
			URL:       &url,
			extraMark: 1,
		}, nil
	}

	return &GitURL{
		URL: &stdurl.URL{
			Scheme: "file",
			Path:   remote,
		},
		extraMark: 2,
	}, nil
}

type RepositoryURL struct {
	GitURL *GitURL

	// if the URL belongs to current Gitea instance, then the below fields have values
	OwnerName     string
	GroupPath     string
	RepoName      string
	RemainingPath string
}

const ExplicitGroupPrefix = "-/group"

func NormalizeGroupPath(groupPath string) string {
	groupPath = strings.Trim(groupPath, "/")
	if s, has := strings.CutPrefix(groupPath, "/"); has {
		return s
	}
	return groupPath
}

func FormatExplicitGroupPath(groupPath string) string {
	groupPath = strings.Trim(NormalizeGroupPath(groupPath), "/")
	if groupPath == "" {
		return ""
	}
	return ExplicitGroupPrefix + "/" + groupPath
}

func parseExplicitGroupPath(path string, ret *RepositoryURL) error {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 5 || segments[1] != "-" || segments[2] != "group" {
		return ErrWrongURLFormat{URL: path}
	}

	ret.OwnerName = segments[0]

	repoIdx := len(segments) - 1
	for i := 3; i < len(segments); i++ {
		if strings.HasSuffix(segments[i], ".git") {
			repoIdx = i
			break
		}
	}
	if repoIdx <= 3 {
		return ErrWrongURLFormat{URL: path}
	}

	ret.GroupPath = strings.Join(segments[3:repoIdx], "/")
	ret.RepoName = strings.TrimSuffix(segments[repoIdx], ".git")
	if ret.RepoName == "" {
		return ErrWrongURLFormat{URL: path}
	}
	if repoIdx+1 < len(segments) {
		ret.RemainingPath = "/" + strings.Join(segments[repoIdx+1:], "/")
	}
	return nil
}

// ParseRepositoryURL tries to parse a Git URL and extract the owner/repository name if it belongs to current Gitea instance.
func ParseRepositoryURL(ctx context.Context, repoURL string) (*RepositoryURL, error) {
	// possible urls for git:
	//  https://my.domain/sub-path/<owner>/<repo>[.git]
	//  git+ssh://user@my.domain/<owner>/<repo>[.git]
	//  ssh://user@my.domain/<owner>/<repo>[.git]
	//  user@my.domain:<owner>/<repo>[.git]
	parsed, err := ParseGitURL(repoURL)
	if err != nil {
		return nil, err
	}

	ret := &RepositoryURL{}
	ret.GitURL = parsed

	fillPathParts := func(s string) error {
		s = strings.TrimPrefix(s, "/")
		fields := strings.Split(s, "/")
		if len(fields) >= 3 && fields[1] == "-" && fields[2] == "group" {
			return parseExplicitGroupPath(s, ret)
		}

		fields = strings.SplitN(s, "/", 3)
		if len(fields) >= 2 {
			ret.OwnerName = fields[0]
			ret.RepoName = strings.TrimSuffix(fields[1], ".git")
			if ret.RepoName == "" {
				return ErrWrongURLFormat{URL: repoURL}
			}
			if len(fields) == 3 {
				ret.RemainingPath = "/" + fields[2]
			}
		}
		return nil
	}

	switch parsed.URL.Scheme {
	case "http", "https":
		if !httplib.IsCurrentGiteaSiteURL(ctx, repoURL) {
			return ret, nil
		}
		if err = fillPathParts(strings.TrimPrefix(parsed.URL.Path, setting.AppSubURL)); err != nil {
			return nil, err
		}
	case "ssh", "git+ssh":
		domainSSH := setting.SSH.Domain
		domainCur := httplib.GuessCurrentHostDomain(ctx)
		urlDomain, _, _ := net.SplitHostPort(parsed.URL.Host)
		urlDomain = util.IfZero(urlDomain, parsed.URL.Host)
		if urlDomain == "" {
			return ret, nil
		}
		// check whether URL domain is the App domain
		domainMatches := domainSSH == urlDomain
		// check whether URL domain is current domain from context
		domainMatches = domainMatches || (domainCur != "" && domainCur == urlDomain)
		if domainMatches {
			if err = fillPathParts(parsed.URL.Path); err != nil {
				return nil, err
			}
		}
	}
	return ret, nil
}

// MakeRepositoryWebLink generates a web link (http/https) for a git repository (by guessing sometimes)
func MakeRepositoryWebLink(repoURL *RepositoryURL) string {
	if repoURL.OwnerName != "" {
		var groupSegment string
		if repoURL.GroupPath != "" {
			groupSegment = strings.Join(util.SliceMap(strings.Split(repoURL.GroupPath, "/"), stdurl.PathEscape), "/") + "/"
		}
		return setting.AppSubURL + "/" + repoURL.OwnerName + "/" + groupSegment + repoURL.RepoName
	}

	// now, let's guess, for example:
	// * git@github.com:owner/submodule.git
	// * https://github.com/example/submodule1.git
	switch repoURL.GitURL.Scheme {
	case "http", "https":
		return strings.TrimSuffix(repoURL.GitURL.String(), ".git")
	case "ssh", "git+ssh":
		hostname, _, _ := net.SplitHostPort(repoURL.GitURL.Host)
		hostname = util.IfZero(hostname, repoURL.GitURL.Host)
		urlPath := strings.TrimSuffix(repoURL.GitURL.Path, ".git")
		urlPath = strings.TrimPrefix(urlPath, "/")
		urlFull := fmt.Sprintf("https://%s/%s", hostname, urlPath)
		urlFull = strings.TrimSuffix(urlFull, "/")
		return urlFull
	}
	return ""
}
