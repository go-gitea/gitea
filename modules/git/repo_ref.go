// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"strings"
)

// GetRefs returns all references of the repository.
func (repo *Repository) GetRefs() ([]*Reference, error) {
	return repo.GetRefsFiltered("")
}

// ListOccurrences lists all refs of the given refType the given commit appears in
// refType should only be a literal "branch" or "tag" and nothing else
func (repo *Repository) ListOccurrences(ctx context.Context, refType string, commitSHA string) ([]string, error) {
	stdout, _, err := NewCommand(ctx, ToTrustedCmdArgs([]string{refType, "--contains"})...).AddDynamicArguments(commitSHA).RunStdString(&RunOpts{Dir: repo.Path})
	return strings.Split(stdout, "\n"), err
}
