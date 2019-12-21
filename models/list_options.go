package models

import (
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/xorm"
)

// ListOptions options to paginate results
type ListOptions struct {
	PageSize int
	Page     int
}

func (opts ListOptions) getPaginatedSession() *xorm.Session {
	opts.setDefaultValues()

	return x.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
}

func (opts ListOptions) setSessionPagination(sess *xorm.Session) *xorm.Session {
	opts.setDefaultValues()

	return sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
}

func (opts ListOptions) setDefaultValues() {
	if opts.PageSize <= 0 || opts.PageSize > setting.UI.ExplorePagingNum {
		opts.PageSize = setting.UI.ExplorePagingNum
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
}
