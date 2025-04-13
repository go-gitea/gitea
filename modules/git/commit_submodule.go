// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

type SubmoduleWebLink struct {
	RepoWebLink, CommitWebLink string
}

// GetSubModules get all the submodules of current revision git tree
func (c *Commit) GetSubModules() (*ObjectCache[*SubModule], error) {
	if c.submoduleCache != nil {
		return c.submoduleCache, nil
	}

	entry, err := c.GetTreeEntryByPath(".gitmodules")
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return nil, nil
		}
		return nil, err
	}

	rd, err := entry.Blob().DataAsync()
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

// GetSubModule get the submodule according entry name
func (c *Commit) GetSubModule(entryName string) (*SubModule, error) {
	modules, err := c.GetSubModules()
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
