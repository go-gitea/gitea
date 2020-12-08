// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package native

import "code.gitea.io/gitea/modules/git/service"

//  __  _   __
// /__ |_) /__
// \_| |   \_|
//

// GetRepositoryDefaultPublicGPGKey returns the default public key for this commit
func (commit *Commit) GetRepositoryDefaultPublicGPGKey(forceUpdate bool) (*service.GPGSettings, error) {
	repo := commit.Repository()
	if repo == nil {
		return nil, nil
	}
	return repo.GetDefaultPublicGPGKey(forceUpdate)
}
