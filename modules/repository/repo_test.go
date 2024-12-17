// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func Test_calcSync(t *testing.T) {
	gitTags := []*git.Tag{
		/*{
			Name: "v0.1.0-beta", //deleted tag
			Object: git.MustIDFromString(""),
		},
		{
			Name: "v0.1.1-beta", //deleted tag but release should not be deleted because it's a release
			Object: git.MustIDFromString(""),
		},
		*/
		{
			Name:   "v1.0.0", // keep as before
			Object: git.MustIDFromString("1006e6e13c73ad3d9e2d5682ad266b5016523485"),
		},
		{
			Name:   "v1.1.0", // retagged with new commit id
			Object: git.MustIDFromString("bbdb7df30248e7d4a26a909c8d2598a152e13868"),
		},
		{
			Name:   "v1.2.0", // new tag
			Object: git.MustIDFromString("a5147145e2f24d89fd6d2a87826384cc1d253267"),
		},
	}

	dbReleases := []*shortRelease{
		{
			ID:      1,
			TagName: "v0.1.0-beta",
			Sha1:    "244758d7da8dd1d9e0727e8cb7704ed4ba9a17c3",
			IsTag:   true,
		},
		{
			ID:      2,
			TagName: "v0.1.1-beta",
			Sha1:    "244758d7da8dd1d9e0727e8cb7704ed4ba9a17c3",
			IsTag:   false,
		},
		{
			ID:      3,
			TagName: "v1.0.0",
			Sha1:    "1006e6e13c73ad3d9e2d5682ad266b5016523485",
		},
		{
			ID:      4,
			TagName: "v1.1.0",
			Sha1:    "53ab18dcecf4152b58328d1f47429510eb414d50",
		},
	}

	inserts, deletes, updates := calcSync(gitTags, dbReleases)
	if assert.Len(t, inserts, 1, "inserts") {
		assert.EqualValues(t, *gitTags[2], *inserts[0], "inserts equal")
	}

	if assert.Len(t, deletes, 1, "deletes") {
		assert.EqualValues(t, 1, deletes[0], "deletes equal")
	}

	if assert.Len(t, updates, 1, "updates") {
		assert.EqualValues(t, *gitTags[1], *updates[0], "updates equal")
	}
}
