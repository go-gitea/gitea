package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

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

func TestHasUserAStopwatch(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	exists, sw, err := HasUserStopwatch(1)
	assert.True(t, exists)
	assert.Equal(t, sw.ID, int64(1))
	assert.NoError(t, err)

	exists, _, err = HasUserStopwatch(3)
	assert.False(t, exists)
	assert.NoError(t, err)
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
