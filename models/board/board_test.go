// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package board

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestIsBoardTypeValid(t *testing.T) {
	const UnknownType Type = 15

	cases := []struct {
		typ   Type
		valid bool
	}{
		{TypeIndividual, false},
		{TypeRepository, true},
		{TypeOrganization, false},
		{UnknownType, false},
	}

	for _, v := range cases {
		assert.Equal(t, v.valid, IsTypeValid(v.typ))
	}
}

func TestFindBoards(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	boards, _, err := FindBoards(db.DefaultContext, SearchOptions{RepoID: 1})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, boards, 1)

	boards, _, err = FindBoards(db.DefaultContext, SearchOptions{RepoID: 3})
	assert.NoError(t, err)

	// 1 value for this repo exists in the fixtures
	assert.Len(t, boards, 1)
}

func TestBoard(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	board := &Board{
		Type:        TypeRepository,
		ColumnType:  BoardTypeBasicKanban,
		Title:       "New Board",
		RepoID:      1,
		CreatedUnix: timeutil.TimeStampNow(),
		CreatorID:   2,
	}

	assert.NoError(t, NewBoard(board))

	_, err := GetBoardByID(db.DefaultContext, board.ID)
	assert.NoError(t, err)

	// Update board
	board.Title = "Updated title"
	assert.NoError(t, UpdateBoard(db.DefaultContext, board))

	boardFromDB, err := GetBoardByID(db.DefaultContext, board.ID)
	assert.NoError(t, err)

	assert.Equal(t, board.Title, boardFromDB.Title)

	assert.NoError(t, ChangeBoardStatus(board, true))

	// Retrieve from DB afresh to check if it is truly closed
	boardFromDB, err = GetBoardByID(db.DefaultContext, board.ID)
	assert.NoError(t, err)

	assert.True(t, boardFromDB.IsClosed)
}
