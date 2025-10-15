// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import "context"

type SubmoduleWebLink struct {
	RepoWebLink, CommitWebLink string
}

// GetSubModules get all the submodules of current revision git tree
func (c *Commit) GetSubModules(ctx context.Context) (*ObjectCache[*SubModule], error) {
	if c.submoduleCache != nil {
		return c.submoduleCache, nil
	}

	entry, err := c.GetTreeEntryByPath(ctx, ".gitmodules")
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return nil, nil
		}
		return nil, err
	}

	rd, err := entry.Blob().DataAsync(ctx)
	if err != nil {
		return nil, err
	}
	defer rd.Close()

	// at the moment we do not strictly limit the size of the .gitmodules file because some users would have huge .gitmodules files (>1MB)
	c.submoduleCache, err = configParseSubModules(rd)
	if err != nil {
		return nil, err
	}
	return c.submoduleCache, nil
}

// GetSubModule gets the submodule by the entry name.
// It returns "nil, nil" if the submodule does not exist, caller should always remember to check the "nil"
func (c *Commit) GetSubModule(ctx context.Context, entryName string) (*SubModule, error) {
	modules, err := c.GetSubModules(ctx)
	if err != nil {
		return nil, err
	}

	if modules != nil {
		if module, has := modules.Get(entryName); has {
			return module, nil
		}
	}
	return nil, nil
}
