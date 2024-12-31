// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/log"
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

func getRefURL(refURL, urlPrefix, repoFullName, sshDomain string) string {
	if refURL == "" {
		return ""
	}

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
// FIXME: template passes AppURL as urlPrefix, it needs to figure out the correct approach (no hard-coded AppURL anymore)
func (sf *CommitSubModuleFile) RefURL(urlPrefix, repoFullName, sshDomain string) string {
	return getRefURL(sf.refURL, urlPrefix, repoFullName, sshDomain)
}

// RefID returns reference ID.
func (sf *CommitSubModuleFile) RefID() string {
	return sf.refID
}

// SubModuleCommit submodule name and commit from a repository
type SubModuleCommit struct {
	Name   string
	Commit string
}

// GetSubmoduleCommits Returns a list of active submodules in the repository
func GetSubmoduleCommits(ctx context.Context, repoPath string) []SubModuleCommit {
	stdoutReader, stdoutWriter := io.Pipe()
	defer func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}()

	go func() {
		stderrBuilder := strings.Builder{}
		err := NewCommand(ctx, "config", "-f", ".gitmodules", "--list", "--name-only").Run(&RunOpts{
			Timeout: -1,
			Dir:     repoPath,
			Stdout:  stdoutWriter,
			Stderr:  &stderrBuilder,
		})

		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderrBuilder.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	var submodules []SubModuleCommit
	bufReader := bufio.NewReader(stdoutReader)

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			break
		}

		if len(line) < len("submodule.x.url\n") ||
			!strings.HasPrefix(line, "submodule.") ||
			!strings.HasSuffix(line, ".url\n") {

			continue
		}

		name := line[len("submodule.") : len(line)-len(".url\n")]
		name = strings.TrimSpace(name)

		if len(name) == 0 {
			log.Debug("Submodule skipped because it has no name")
			continue
		}

		// If no commit was found for the module skip it
		commit, _, err := NewCommand(ctx, "submodule", "status").AddDynamicArguments(name).RunStdString(&RunOpts{Dir: repoPath})
		if err != nil {
			log.Debug("Submodule %s skipped because it has no commit", name)
			continue
		}

		if len(commit) > 0 {
			commit = commit[1:]
		}

		fields := strings.Fields(commit)

		if len(fields) == 0 {
			log.Debug("Submodule %s skipped because it has no valid commit", name)
			continue
		}

		commit = fields[0]

		if len(commit) != 40 {
			log.Debug("Submodule %s skipped due to malformed commit hash", name)
			continue
		}

		submodules = append(submodules, SubModuleCommit{name, commit})
	}

	return submodules
}

// AddSubmoduleIndexes Adds the given submodules to the git index. Requires the .gitmodules file to be already present.
func AddSubmoduleIndexes(ctx context.Context, repoPath string, submodules []SubModuleCommit) error {
	for _, submodule := range submodules {
		if stdout, _, err := NewCommand(ctx, "update-index", "--add", "--cacheinfo", "160000").AddDynamicArguments(submodule.Commit, submodule.Name).
			RunStdString(&RunOpts{Dir: repoPath}); err != nil {
			log.Error("Unable to add %s as submodule to repo %s: stdout %s\nError: %v", submodule.Name, repoPath, stdout, err)
			return err
		}
	}

	return nil
}
