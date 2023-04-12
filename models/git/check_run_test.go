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

	_, err = CreateCheckRun(db.DefaultContext, &NewCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		HeadSHA: "1234123412341234123412341234123412341234",
		Name:    "test 1",
	})
	assert.EqualValues(t, ErrCheckRunExist{RepoID: 1, HeadSHA: "1234123412341234123412341234123412341234", Name: "test 1"}, err)

	title := "build output title"
	summary := "build output summary"
	boolTrue := true

	checkRun, err := CreateCheckRun(db.DefaultContext, &NewCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		HeadSHA: "1234123412341234123412341234123412341234",
		Name:    "test with output",
		Output: &structs.CheckRunOutput{
			Title:   &title,
			Summary: &summary,
			Annotations: []*structs.CheckRunAnnotation{
				{
					Title:           "hello world",
					Message:         "test message",
					AnnotationLevel: structs.CheckRunAnnotationLevelWarning,
				},
				{
					Title:           "hello world 2",
					Message:         "test message",
					AnnotationLevel: structs.CheckRunAnnotationLevelWarning,
					DeleteMark:      &boolTrue,
				},
			},
		},
	})
	assert.NoError(t, err)

	checkRun = unittest.AssertExistsAndLoadBean(t, &CheckRun{ID: checkRun.ID})
	assert.Equal(t, "test with output", checkRun.Name)
	assert.NoError(t, checkRun.LoadOutput(db.DefaultContext))
	assert.NoError(t, checkRun.LoadOutput(db.DefaultContext))
	assert.NotNil(t, checkRun.Output)
	assert.Equal(t, 1, len(checkRun.Output.Annotations))
	assert.EqualValues(t, "hello world", checkRun.Output.Annotations[0].Title)
	assert.EqualValues(t, title, checkRun.Output.Title)
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
			wantErr: errors.New("`repo` or `creator` not set"),
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
			wantErr: ErrUnVaildCheckRunOptions{"request `started_at` if staus isn't `queued`"},
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

	checkRun, err := GetCheckRunByRepoIDAndID(db.DefaultContext, 1, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, "test check run", checkRun.Name)
	assert.EqualValues(t, structs.CommitStatusSuccess, checkRun.ToStatus(nil).State)

	assert.NoError(t, checkRun.LoadOutput(db.DefaultContext))
	assert.NotNil(t, checkRun.Output)
	assert.Equal(t, 2, len(checkRun.Output.Annotations))

	_, err = GetCheckRunByRepoIDAndID(db.DefaultContext, 1, 5)
	assert.EqualValues(t, ErrCheckRunNotExist{RepoID: 1, ID: 5}, err)
}

func TestUpdateCheckRunOptions_Vaild(t *testing.T) {
	creator := &user_model.User{}
	repo := &repo_model.Repository{}
	tests := []struct {
		checkRun *CheckRun
		opts     *UpdateCheckRunOptions
		wantErr  error
	}{
		// test 1
		{
			checkRun: &CheckRun{},
			opts:     &UpdateCheckRunOptions{},
			wantErr:  errors.New("`repo` or `creator` not set"),
		},
		// test 2
		{
			checkRun: &CheckRun{},
			opts: &UpdateCheckRunOptions{
				Repo:    repo,
				Creator: creator,
				Status:  structs.CheckRunStatusInProgress,
			},
			wantErr: ErrUnVaildCheckRunOptions{"request `started_at` if staus isn't `queued`"},
		},
		// test 3
		{
			checkRun: &CheckRun{},
			opts: &UpdateCheckRunOptions{
				StartedAt: timeutil.TimeStampNow().AddDuration(-time.Minute * 5),
				Repo:      repo,
				Creator:   creator,
				Status:    structs.CheckRunStatusInProgress,
			},
		},
		// test 4
		{
			checkRun: &CheckRun{},
			opts: &UpdateCheckRunOptions{
				Repo:      repo,
				Creator:   creator,
				Status:    structs.CheckRunStatusInProgress,
				StartedAt: timeutil.TimeStampNow().AddDuration(-time.Minute * 5),
			},
		},
		// test 5
		{
			checkRun: &CheckRun{
				StartedAt: timeutil.TimeStampNow().AddDuration(-time.Minute * 5),
			},
			opts: &UpdateCheckRunOptions{
				Repo:    repo,
				Creator: creator,
				Status:  structs.CheckRunStatusCompleted,
			},
			wantErr: ErrUnVaildCheckRunOptions{"request `conclusion` if staus is `completed`"},
		},
		// test 6
		{
			checkRun: &CheckRun{
				StartedAt:  timeutil.TimeStampNow().AddDuration(-time.Minute * 5),
				Conclusion: CheckRunConclusionFailure,
			},
			opts: &UpdateCheckRunOptions{
				Repo:    repo,
				Creator: creator,
				Status:  structs.CheckRunStatusCompleted,
			},
		},
	}
	for _, tt := range tests {
		err := tt.opts.Vaild(tt.checkRun)
		if tt.wantErr == nil {
			assert.NoError(t, err)
		} else {
			assert.EqualValues(t, tt.wantErr, err)
		}
	}
}

func TestUpdate_Update(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	checkRun2 := unittest.AssertExistsAndLoadBean(t, &CheckRun{ID: 2})
	checkRun1 := unittest.AssertExistsAndLoadBean(t, &CheckRun{ID: 1})

	// 1. test update name
	err := checkRun2.Update(db.DefaultContext, UpdateCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		Name:    "test check run",
	})
	assert.EqualValues(t, ErrCheckRunExist{RepoID: 1,
		HeadSHA: "1234123412341234123412341234123412341234",
		Name:    "test check run"}, err)

	err = checkRun2.Update(db.DefaultContext, UpdateCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		Name:    "test check run 4",
	})
	assert.NoError(t, err)
	assert.Equal(t, "test check run 4", checkRun2.Name)

	// 2. update other value
	err = checkRun2.Update(db.DefaultContext, UpdateCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		Status:  structs.CheckRunStatusCompleted,
	})
	assert.EqualValues(t, ErrUnVaildCheckRunOptions{"request `conclusion` if staus is `completed`"}, err)

	url := "https://example.com/builds/"
	extID := "22"

	err = checkRun2.Update(db.DefaultContext, UpdateCheckRunOptions{
		Repo:        repo1,
		Creator:     user2,
		Status:      structs.CheckRunStatusCompleted,
		StartedAt:   timeutil.TimeStampNow().AddDuration(-5 * time.Second),
		CompletedAt: timeutil.TimeStampNow(),
		ExternalID:  &extID,
		DetailsURL:  &url,
		Conclusion:  structs.CheckRunConclusionFailure,
	})
	assert.NoError(t, err)
	assert.Equal(t, "test check run 4", checkRun2.Name)
	assert.NotEqualValues(t, timeutil.TimeStamp(0), checkRun2.CompletedAt)

	// 2. update outputs
	assert.NoError(t, checkRun1.LoadOutput(db.DefaultContext))
	assert.NotNil(t, checkRun1.Output)
	assert.Equal(t, 2, len(checkRun1.Output.Annotations))

	title := "test ci output changed"
	boolTrue := true
	err = checkRun1.Update(db.DefaultContext, UpdateCheckRunOptions{
		Repo:    repo1,
		Creator: user2,
		Output: &structs.CheckRunOutput{
			Title: &title,
			Annotations: []*structs.CheckRunAnnotation{
				{
					Title:      checkRun1.Output.Annotations[0].Title,
					DeleteMark: &boolTrue, // delete first item
				},
				{
					Title:      checkRun1.Output.Annotations[1].Title,
					Message:    "test message 2 changed",
					AppendMark: &boolTrue, // mark as appended, so willn't be changed
				},
				{
					Title:           "title 3",
					Message:         "test message 3",
					AnnotationLevel: structs.CheckRunAnnotationLevelNotice,
				},
			},
		},
	})
	assert.NoError(t, err)

	checkRun1 = unittest.AssertExistsAndLoadBean(t, &CheckRun{ID: 1})
	assert.NoError(t, checkRun1.LoadOutput(db.DefaultContext))
	assert.NotNil(t, checkRun1.Output)
	assert.Equal(t, 2, len(checkRun1.Output.Annotations))
	assert.EqualValues(t, "title 3", checkRun1.Output.Annotations[0].Title)
}
