package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestGetStopwatchByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	sw, err := GetStopwatchByID(1)
	assert.Equal(t, sw.CreatedUnix, int64(1500988502))
	assert.Equal(t, sw.UserID, int64(1))
	// Tue Jul 25 13:15:02 2017 UTC
	assert.Equal(t, sw.Created, time.Unix(1500988502, 0))
	assert.NoError(t, err)

	sw, err = GetStopwatchByID(3)
	assert.Error(t, err)
	assert.Equal(t, true, IsErrStopwatchNotExist(err))
}

func TestCancelStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	err := CancelStopwatch(1, 1)
	assert.NoError(t, err)
	AssertNotExistsBean(t, &Stopwatch{UserID: 1, IssueID: 1})

	_ = AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeCancelTracking, PosterID: 1, IssueID: 1})

	assert.Nil(t, CancelStopwatch(1, 2))
}

func TestStopwatchExists(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.True(t, StopwatchExists(1, 1))
	assert.False(t, StopwatchExists(1, 2))
}

func TestCreateOrStopIssueStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	assert.NoError(t, CreateOrStopIssueStopwatch(3, 1))
	sw := AssertExistsAndLoadBean(t, &Stopwatch{UserID: 3, IssueID: 1}).(*Stopwatch)
	assert.Equal(t, true, sw.Created.Before(time.Now()))

	assert.NoError(t, CreateOrStopIssueStopwatch(2, 2))
	AssertNotExistsBean(t, &Stopwatch{UserID: 2, IssueID: 2})
	AssertExistsAndLoadBean(t, &TrackedTime{UserID: 2, IssueID: 2})
}
