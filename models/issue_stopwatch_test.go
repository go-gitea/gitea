package models

import (
	"testing"

	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestCancelStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user1, err := GetUserByID(1)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)
	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)

	err = CancelStopwatch(user1, issue1)
	assert.NoError(t, err)
	AssertNotExistsBean(t, &Stopwatch{UserID: user1.ID, IssueID: issue1.ID})

	_ = AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeCancelTracking, PosterID: user1.ID, IssueID: issue1.ID})

	assert.Nil(t, CancelStopwatch(user1, issue2))
}

func TestStopwatchExists(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.True(t, StopwatchExists(1, 1))
	assert.False(t, StopwatchExists(1, 2))
}

func TestHasUserStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	exists, sw, err := HasUserStopwatch(1)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int64(1), sw.ID)

	exists, _, err = HasUserStopwatch(3)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCreateOrStopIssueStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2, err := GetUserByID(2)
	assert.NoError(t, err)
	user3, err := GetUserByID(3)
	assert.NoError(t, err)

	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)
	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)

	assert.NoError(t, CreateOrStopIssueStopwatch(user3, issue1))
	sw := AssertExistsAndLoadBean(t, &Stopwatch{UserID: 3, IssueID: 1}).(*Stopwatch)
	assert.Equal(t, true, sw.CreatedUnix <= timeutil.TimeStampNow())

	assert.NoError(t, CreateOrStopIssueStopwatch(user2, issue2))
	AssertNotExistsBean(t, &Stopwatch{UserID: 2, IssueID: 2})
	AssertExistsAndLoadBean(t, &TrackedTime{UserID: 2, IssueID: 2})
}
