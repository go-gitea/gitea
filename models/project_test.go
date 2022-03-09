// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestIsProjectTypeValid(t *testing.T) {
	const UnknownType ProjectType = 15

	cases := []struct {
		typ   ProjectType
		valid bool
	}{
		{ProjectTypeIndividual, false},
		{ProjectTypeRepository, true},
		{ProjectTypeOrganization, false},
		{UnknownType, false},
	}

	for _, v := range cases {
		assert.Equal(t, v.valid, IsProjectTypeValid(v.typ))
	}
}

func TestGetProjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	projects, _, err := GetProjects(ProjectSearchOptions{RepoID: 1})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)

	projects, _, err = GetProjects(ProjectSearchOptions{RepoID: 3})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)
}

func TestProject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project := &Project{
		Type:        ProjectTypeRepository,
		BoardType:   ProjectBoardTypeBasicKanban,
		Title:       "New Project",
		RepoID:      1,
		CreatedUnix: timeutil.TimeStampNow(),
		CreatorID:   2,
	}

	assert.NoError(t, NewProject(project))

	_, err := GetProjectByID(project.ID)
	assert.NoError(t, err)

	// Update project
	project.Title = "Updated title"
	assert.NoError(t, UpdateProject(project))

	projectFromDB, err := GetProjectByID(project.ID)
	assert.NoError(t, err)

	assert.Equal(t, project.Title, projectFromDB.Title)

	assert.NoError(t, ChangeProjectStatus(project, true))

	// Retrieve from DB afresh to check if it is truly closed
	projectFromDB, err = GetProjectByID(project.ID)
	assert.NoError(t, err)

	assert.True(t, projectFromDB.IsClosed)
}
