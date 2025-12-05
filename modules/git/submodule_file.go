// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

// SubmoduleFile represents a file with submodule type.
type SubmoduleFile struct {
	repoLink string
	fullPath string
	refURL   string
	refID    string

	parsed           bool
	parsedTargetLink string
}

// NewSubmoduleFile create a new submodule file
func NewSubmoduleFile(repoLink, fullPath, refURL, refID string) *SubmoduleFile {
	return &SubmoduleFile{repoLink: repoLink, fullPath: fullPath, refURL: refURL, refID: refID}
}

// RefID returns the commit ID of the submodule, it returns empty string for nil receiver
func (sf *SubmoduleFile) RefID() string {
	if sf == nil {
		return ""
	}
	return sf.refID
}

func (sf *SubmoduleFile) FullPath() string {
	return sf.fullPath
}

type SubmoduleWebLink struct {
	RepoWebLink, CommitWebLink string
}

func (sf *SubmoduleFile) getWebLinkInTargetRepo(ctx context.Context, moreLinkPath string) *SubmoduleWebLink {
	if sf == nil || sf.refURL == "" {
		return nil
	}
	if strings.HasPrefix(sf.refURL, "../") {
		targetLink := path.Join(sf.repoLink, sf.refURL)
		return &SubmoduleWebLink{RepoWebLink: targetLink, CommitWebLink: targetLink + moreLinkPath}
	}
	if !sf.parsed {
		sf.parsed = true
		parsedURL, err := giturl.ParseRepositoryURL(ctx, sf.refURL)
		if err != nil {
			return nil
		}
		sf.parsedTargetLink = giturl.MakeRepositoryWebLink(parsedURL)
	}
	return &SubmoduleWebLink{RepoWebLink: sf.parsedTargetLink, CommitWebLink: sf.parsedTargetLink + moreLinkPath}
}

// SubmoduleWebLinkTree tries to make the submodule's tree link in its own repo, it also works on "nil" receiver
// It returns nil if the submodule does not have a valid URL or is nil
func (sf *SubmoduleFile) SubmoduleWebLinkTree(ctx context.Context, optCommitID ...string) *SubmoduleWebLink {
	return sf.getWebLinkInTargetRepo(ctx, "/tree/"+util.OptionalArg(optCommitID, sf.RefID()))
}

// SubmoduleWebLinkCompare tries to make the submodule's compare link in its own repo, it also works on "nil" receiver
// It returns nil if the submodule does not have a valid URL or is nil
func (sf *SubmoduleFile) SubmoduleWebLinkCompare(ctx context.Context, commitID1, commitID2 string) *SubmoduleWebLink {
	return sf.getWebLinkInTargetRepo(ctx, "/compare/"+commitID1+"..."+commitID2)
}

// GetRepoSubmoduleFiles returns a list of submodules paths and their commits from a repository
// This function is only for generating new repos based on existing template, the template couldn't be too large.
func GetRepoSubmoduleFiles(ctx context.Context, repoPath, refName string) (submoduleFiles []SubmoduleFile, _ error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	err = gitcmd.NewCommand("ls-tree", "-r", "--").
		AddDynamicArguments(refName).
		WithDir(repoPath).
		WithStdout(stdoutWriter).
		WithPipelineFunc(func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer stdoutReader.Close()

			scanner := bufio.NewScanner(stdoutReader)
			for scanner.Scan() {
				entry, err := parseLsTreeLine(scanner.Bytes())
				if err != nil {
					cancel()
					return err
				}
				if entry.EntryMode == EntryModeCommit {
					submoduleFiles = append(submoduleFiles, SubmoduleFile{fullPath: entry.Name, refID: entry.ID.String()})
				}
			}
			return scanner.Err()
		}).
		Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetTemplateSubmoduleCommits: error running git ls-tree: %v", err)
	}
	return submoduleFiles, nil
}

// AddSubmodulesToRepoIndex Adds the given submodules to the git index.
// It is only for generating new repos based on existing template, requires the .gitmodules file to be already present in the work dir.
func AddSubmodulesToRepoIndex(ctx context.Context, repoPath string, submodules []SubmoduleFile) error {
	for _, submodule := range submodules {
		cmd := gitcmd.NewCommand("update-index", "--add", "--cacheinfo", "160000").AddDynamicArguments(submodule.RefID(), submodule.fullPath)
		if stdout, _, err := cmd.WithDir(repoPath).RunStdString(ctx); err != nil {
			log.Error("Unable to add %s as submodule to repo %s: stdout %s\nError: %v", submodule.fullPath, repoPath, stdout, err)
			return err
		}
	}
	return nil
}
