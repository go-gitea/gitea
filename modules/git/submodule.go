package git

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"code.gitea.io/gitea/modules/log"
)

// SubModuleCommit submodule name and commit from a repository
type SubModuleCommit struct {
	Path   string
	Commit string
}

// GetSubmoduleCommits returns a list of submodules paths and their commits from a repository
// This function is only for generating new repos based on existing template, the template couldn't be too large.
func GetSubmoduleCommits(ctx context.Context, repoPath string) (submoduleCommits []SubModuleCommit, _ error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	opts := &RunOpts{
		Dir:    repoPath,
		Stdout: stdoutWriter,
		PipelineFunc: func(ctx context.Context, cancel context.CancelFunc) error {
			_ = stdoutWriter.Close()
			defer stdoutReader.Close()

			scanner := bufio.NewScanner(stdoutReader)
			for scanner.Scan() {
				entry, err := parseLsTreeLine(scanner.Bytes())
				if err != nil {
					cancel()
					return err
				}
				if entry.IsSubModule() {
					submoduleCommits = append(submoduleCommits, SubModuleCommit{Path: entry.Name(), Commit: entry.ID.String()})
				}
			}
			return scanner.Err()
		},
	}
	err = NewCommand(ctx, "ls-tree", "-r", "--", "HEAD").Run(opts)
	if err != nil {
		return nil, fmt.Errorf("GetSubmoduleCommits: error running git ls-tree: %v", err)
	}
	return submoduleCommits, nil
}

// AddSubmoduleIndexes Adds the given submodules to the git index. Requires the .gitmodules file to be already present.
func AddSubmoduleIndexes(ctx context.Context, repoPath string, submodules []SubModuleCommit) error {
	for _, submodule := range submodules {
		cmd := NewCommand(ctx, "update-index", "--add", "--cacheinfo", "160000").AddDynamicArguments(submodule.Commit, submodule.Path)
		if stdout, _, err := cmd.RunStdString(&RunOpts{Dir: repoPath}); err != nil {
			log.Error("Unable to add %s as submodule to repo %s: stdout %s\nError: %v", submodule.Path, repoPath, stdout, err)
			return err
		}
	}
	return nil
}
