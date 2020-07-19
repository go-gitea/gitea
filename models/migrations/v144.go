package migrations

import "xorm.io/xorm"

func addAutoMergeTable(x *xorm.Engine) error {
	type MergeStyle string
	type ScheduledPullRequestMerge struct {
		ID         int64      `xorm:"pk autoincr"`
		PullID     int64      `xorm:"BIGINT"`
		UserID     int64      `xorm:"BIGINT"`
		MergeStyle MergeStyle `xorm:"varchar(50)"`
		Message    string     `xorm:"TEXT"`
	}

	return x.Sync2(&ScheduledPullRequestMerge{})
}
