package group

import (
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
)

// Group represents a group of repositories for a user or organization
type Group struct {
	ID          int64 `xorm:"pk autoincr"`
	OwnerID     int64 `xorm:"UNIQUE(s) index"`
	OwnerName   string
	Owner       *user_model.User `xorm:"-"`
	LowerName   string           `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Name        string           `xorm:"INDEX NOT NULL"`
	Description string           `xorm:"TEXT"`

	ParentGroupID int64    `xorm:"DEFAULT NULL"`
	SubGroups     []*Group `xorm:"-"`
}

func (Group) TableName() string { return "repo_group" }

func init() {
	db.RegisterModel(new(Group))
}
