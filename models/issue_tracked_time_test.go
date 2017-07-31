package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddTime(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	//3661 = 1h 1min 1s
	assert.NoError(t, AddTime(3, 1, 3661))
	tt := AssertExistsAndLoadBean(t, &TrackedTime{UserID: 3, IssueID: 1}).(*TrackedTime)
	assert.Equal(t, tt.Time, int64(3661))

	comment := AssertExistsAndLoadBean(t, &Comment{Type: CommentTypeAddTimeManual, PosterID: 3, IssueID: 1}).(*Comment)
	assert.Equal(t, comment.Content, "1h 1min 1s")
}

func TestGetTrackedTimes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// by Issue
	times, err := GetTrackedTimes(FindTrackedTimesOptions{IssueID: 1})
	assert.Len(t, times, 1)
	assert.Equal(t, times[0].Time, int64(400))
	assert.NoError(t, err)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{IssueID: -1})
	assert.Len(t, times, 0)
	assert.NoError(t, err)

	// by User
	times, err = GetTrackedTimes(FindTrackedTimesOptions{UserID: 1})
	assert.Len(t, times, 1)
	assert.Equal(t, times[0].Time, int64(400))
	assert.NoError(t, err)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{UserID: 3})
	assert.Len(t, times, 0)
	assert.NoError(t, err)

	// by Repo
	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 2})
	assert.Len(t, times, 1)
	assert.Equal(t, times[0].Time, int64(1))
	assert.NoError(t, err)
	issue, err := GetIssueByID(times[0].IssueID)
	assert.NoError(t, err)
	assert.Equal(t, issue.RepoID, int64(2))

	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 1})
	assert.Len(t, times, 4)
	assert.NoError(t, err)

	times, err = GetTrackedTimes(FindTrackedTimesOptions{RepositoryID: 10})
	assert.Len(t, times, 0)
	assert.NoError(t, err)
}

func TestTotalTimes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	total, err := TotalTimes(FindTrackedTimesOptions{IssueID: 1})
	assert.Len(t, total, 1)
	for user, time := range total {
		assert.Equal(t, int64(1), user.ID)
		assert.Equal(t, "6min 40s", time)
	}
	assert.NoError(t, err)

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 2})
	assert.Len(t, total, 1)
	for user, time := range total {
		assert.Equal(t, int64(2), user.ID)
		assert.Equal(t, "1h 1min 2s", time)
	}
	assert.NoError(t, err)

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 5})
	assert.Len(t, total, 1)
	for user, time := range total {
		assert.Equal(t, int64(2), user.ID)
		assert.Equal(t, "1s", time)
	}
	assert.NoError(t, err)

	total, err = TotalTimes(FindTrackedTimesOptions{IssueID: 4})
	assert.Len(t, total, 0)
	assert.NoError(t, err)
}
