package git

import (
	"errors"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestCreateCheckRun(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	_, err := CreateCheckRun(db.DefaultContext, &NewCheckRunOptions{})
	assert.NotNil(t, err)

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	_, err = CreateCheckRun(db.DefaultContext, &NewCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		HeadSHA: "1234",
		Name:    "test 1",
	})
	assert.NotNil(t, err)

	run, err := CreateCheckRun(db.DefaultContext, &NewCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		HeadSHA: "1234123412341234123412341234123412341234",
		Name:    "test 1",
	})
	assert.NoError(t, err)
	assert.NotNil(t, run)
}

func TestNewCheckRunOptions_Vaild(t *testing.T) {
	creator := &user_model.User{}
	repo := &repo_model.Repository{}

	tests := []struct {
		opts    *NewCheckRunOptions
		wantErr error
	}{
		// test 1
		{
			opts:    &NewCheckRunOptions{},
			wantErr: errors.New("`repo` or `creater` not set"),
		},
		// test 2
		{
			opts: &NewCheckRunOptions{
				Creator: creator,
				Repo:    repo,
			},
			wantErr: ErrUnVaildCheckRunOptions{Err: "request `name`"},
		},
		// test 3
		{
			opts: &NewCheckRunOptions{
				Creator: creator,
				Repo:    repo,
				Name:    "test name",
			},
			wantErr: ErrUnVaildCheckRunOptions{Err: "request `head_sha`"},
		},
		// test 4
		{
			opts: &NewCheckRunOptions{
				Creator: creator,
				Repo:    repo,
				Name:    "test name",
				HeadSHA: "11122333",
			},
		},
		// test 5
		{
			opts: &NewCheckRunOptions{
				Creator: creator,
				Repo:    repo,
				Name:    "test name",
				HeadSHA: "11122333",
				Status:  structs.CheckRunStatusInProgress,
			},
			wantErr: ErrUnVaildCheckRunOptions{"request `start_at` if staus isn't `queued`"},
		},
		// test 6
		{
			opts: &NewCheckRunOptions{
				Creator:   creator,
				Repo:      repo,
				Name:      "test name",
				HeadSHA:   "11122333",
				Status:    structs.CheckRunStatusInProgress,
				StartedAt: timeutil.TimeStampNow().AddDuration(-10 * time.Second),
			},
		},
		// test 7
		{
			opts: &NewCheckRunOptions{
				Creator:   creator,
				Repo:      repo,
				Name:      "test name",
				HeadSHA:   "11122333",
				Status:    structs.CheckRunStatusCompleted,
				StartedAt: timeutil.TimeStampNow().AddDuration(-10 * time.Second),
			},
			wantErr: ErrUnVaildCheckRunOptions{"request `conclusion` if staus is `completed`"},
		},
		// test 8
		{
			opts: &NewCheckRunOptions{
				Creator:    creator,
				Repo:       repo,
				Name:       "test name",
				HeadSHA:    "11122333",
				Status:     structs.CheckRunStatusCompleted,
				StartedAt:  timeutil.TimeStampNow().AddDuration(-10 * time.Second),
				Conclusion: structs.CheckRunConclusionFailure,
			},
		},
	}
	for _, tt := range tests {
		err := tt.opts.Vaild()
		if tt.wantErr == nil {
			assert.NoError(t, err)
		} else {
			assert.EqualValues(t, tt.wantErr, err)
		}
	}
}

func TestGetCheckRunByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	checkRun, err := GetCheckRunByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, "test check run", checkRun.Name)
	assert.EqualValues(t, structs.CommitStatusSuccess, checkRun.ToStatus(nil).State)

	_, err = GetCheckRunByID(db.DefaultContext, 5)
	assert.EqualValues(t, ErrCheckRunNotExist{ID: 5}, err)
}
