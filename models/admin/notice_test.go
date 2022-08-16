// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin_test

import (
	"testing"

	"code.gitea.io/gitea/models/admin"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestNotice_TrStr(t *testing.T) {
	notice := &admin.Notice{
		Type:        admin.NoticeRepository,
		Description: "test description",
	}
	assert.Equal(t, "admin.notices.type_1", notice.TrStr())
}

func TestCreateNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	noticeBean := &admin.Notice{
		Type:        admin.NoticeRepository,
		Description: "test description",
	}
	unittest.AssertNotExistsBean(t, noticeBean)
	assert.NoError(t, admin.CreateNotice(db.DefaultContext, noticeBean.Type, noticeBean.Description))
	unittest.AssertExistsAndLoadBean(t, noticeBean)
}

func TestCreateRepositoryNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	noticeBean := &admin.Notice{
		Type:        admin.NoticeRepository,
		Description: "test description",
	}
	unittest.AssertNotExistsBean(t, noticeBean)
	assert.NoError(t, admin.CreateRepositoryNotice(noticeBean.Description))
	unittest.AssertExistsAndLoadBean(t, noticeBean)
}

// TODO TestRemoveAllWithNotice

func TestCountNotices(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.Equal(t, int64(3), admin.CountNotices())
}

func TestNotices(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	notices, err := admin.Notices(1, 2)
	assert.NoError(t, err)
	if assert.Len(t, notices, 2) {
		assert.Equal(t, int64(3), notices[0].ID)
		assert.Equal(t, int64(2), notices[1].ID)
	}

	notices, err = admin.Notices(2, 2)
	assert.NoError(t, err)
	if assert.Len(t, notices, 1) {
		assert.Equal(t, int64(1), notices[0].ID)
	}
}

func TestDeleteNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
	assert.NoError(t, admin.DeleteNotice(3))
	unittest.AssertNotExistsBean(t, &admin.Notice{ID: 3})
}

func TestDeleteNotices(t *testing.T) {
	// delete a non-empty range
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
	assert.NoError(t, admin.DeleteNotices(1, 2))
	unittest.AssertNotExistsBean(t, &admin.Notice{ID: 1})
	unittest.AssertNotExistsBean(t, &admin.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
}

func TestDeleteNotices2(t *testing.T) {
	// delete an empty range
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
	assert.NoError(t, admin.DeleteNotices(3, 2))
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
}

func TestDeleteNoticesByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 3})
	assert.NoError(t, admin.DeleteNoticesByIDs([]int64{1, 3}))
	unittest.AssertNotExistsBean(t, &admin.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &admin.Notice{ID: 2})
	unittest.AssertNotExistsBean(t, &admin.Notice{ID: 3})
}
