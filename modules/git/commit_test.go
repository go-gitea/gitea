// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	commitsCount, err := CommitsCount(bareRepo1Path, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestGetFullCommitID(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	id, err := GetFullCommitID(bareRepo1Path, "8006ff9a")
	assert.NoError(t, err)
	assert.Equal(t, "8006ff9adbf0cb94da7dad9e537e53817f9fa5c0", id)
}

func TestGetFullCommitIDError(t *testing.T) {
	bareRepo1Path := filepath.Join(testReposDir, "repo1_bare")

	id, err := GetFullCommitID(bareRepo1Path, "unknown")
	assert.Empty(t, id)
	if assert.Error(t, err) {
		assert.EqualError(t, err, "object does not exist [id: unknown, rel_path: ]")
	}
}

func TestCommitFromReader(t *testing.T) {
	gitCatFileBatch := `gpgsig -----BEGIN PGP SIGNATURE-----

 wsBcBAABCAAQBQJf1RMlCRBK7hj4Ov3rIwAAdHIIAGknVUi+8Fww7D+DtHlCVzcs
 8t068qrNAifGfNPnvKKDhvEq850UCL01kTNhOnMu7qtFao9BUMAzWvYQEiHjp+BW
 x2seyGdFqD0a4laRzUSLllpbDpk5oWJvmuIW2aVxojHo4FwrnSGlkIMKM8aXD4f+
 FWR4c2X2Ik1drEUo0v0k12RrVhI77aXn38sUz3VyDrm48I+IBbBP5+nK5GyvGDIQ
 CVx6Plz3OziTuUfpc3lixjT6EjypdCTkO0WPZemdfHGWxP0vTqqsmdlBhGMy5+I8
 vIKQIxeC2yEP6R7x711darildz1Qux7PiH/R8JUH9I7Pkmmm1c0AbsD0Tyg37UM=
 =v3Ra
 -----END PGP SIGNATURE-----`

	gitCatFileBatchreader := strings.NewReader(gitCatFileBatch)
	commit, err := CommitFromReader(nil, SHA1{}, gitCatFileBatchreader)
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	assert.EqualValues(t, `
 wsBcBAABCAAQBQJf1RMlCRBK7hj4Ov3rIwAAdHIIAGknVUi+8Fww7D+DtHlCVzcs
 8t068qrNAifGfNPnvKKDhvEq850UCL01kTNhOnMu7qtFao9BUMAzWvYQEiHjp+BW
 x2seyGdFqD0a4laRzUSLllpbDpk5oWJvmuIW2aVxojHo4FwrnSGlkIMKM8aXD4f+
 FWR4c2X2Ik1drEUo0v0k12RrVhI77aXn38sUz3VyDrm48I+IBbBP5+nK5GyvGDIQ
 CVx6Plz3OziTuUfpc3lixjT6EjypdCTkO0WPZemdfHGWxP0vTqqsmdlBhGMy5+I8
 vIKQIxeC2yEP6R7x711darildz1Qux7PiH/R8JUH9I7Pkmmm1c0AbsD0Tyg37UM=
 =v3Ra
 -----END PGP SIGNATURE-----`, commit.Signature.Payload)
}
