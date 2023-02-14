// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package project

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestIsProjectTypeValid(t *testing.T) {
	const UnknownType Type = 15

	cases := []struct {
		typ   Type
		valid bool
	}{
		{TypeIndividual, false},
		{TypeRepository, true},
		{TypeOrganization, true},
		{UnknownType, false},
	}

	for _, v := range cases {
		assert.Equal(t, v.valid, IsTypeValid(v.typ))
	}
}

func TestGetProjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	projects, _, err := FindProjects(db.DefaultContext, SearchOptions{RepoID: 1})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)

	projects, _, err = FindProjects(db.DefaultContext, SearchOptions{RepoID: 3})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, projects, 1)
}

func TestProject(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	project := &Project{
		Type:        TypeRepository,
		BoardType:   BoardTypeBasicKanban,
		CardType:    CardTypeTextOnly,
		Title:       "New Project",
		RepoID:      1,
		CreatedUnix: timeutil.TimeStampNow(),
		CreatorID:   2,
	}

	assert.NoError(t, NewProject(project))

	_, err := GetProjectByID(db.DefaultContext, project.ID)
	assert.NoError(t, err)

	// Update project
	project.Title = "Updated title"
	assert.NoError(t, UpdateProject(db.DefaultContext, project))

	projectFromDB, err := GetProjectByID(db.DefaultContext, project.ID)
	assert.NoError(t, err)

	assert.Equal(t, project.Title, projectFromDB.Title)

	assert.NoError(t, ChangeProjectStatus(project, true))

	// Retrieve from DB afresh to check if it is truly closed
	projectFromDB, err = GetProjectByID(db.DefaultContext, project.ID)
	assert.NoError(t, err)

	assert.True(t, projectFromDB.IsClosed)
}
