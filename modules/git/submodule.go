// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

type SubmoduleWebLink struct {
	RepoWebLink, CommitWebLink string
}

// GetSubModules get all the submodules of current revision git tree
func (t *Tree) GetSubModules() (*ObjectCache[*SubModule], error) {
	if t.submoduleCache != nil {
		return t.submoduleCache, nil
	}

	entry, err := t.GetTreeEntryByPath(".gitmodules")
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
	t.submoduleCache, err = configParseSubModules(rd)
	if err != nil {
		return nil, err
	}
	return t.submoduleCache, nil
}

// GetSubModule gets the submodule by the entry name.
// It returns "nil, nil" if the submodule does not exist, caller should always remember to check the "nil"
func (t *Tree) GetSubModule(entryName string) (*SubModule, error) {
	modules, err := t.GetSubModules()
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
