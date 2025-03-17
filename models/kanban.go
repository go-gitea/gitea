// models/kanban.go
package models

type KanbanBoard struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"INDEX NOT NULL"`
	Title     string `xorm:"NOT NULL"`
	CreatedAt int64  `xorm:"created"`
	UpdatedAt int64  `xorm:"updated"`
}

type KanbanColumn struct {
	ID        int64  `xorm:"pk autoincr"`
	BoardID   int64  `xorm:"INDEX NOT NULL"`
	Title     string `xorm:"NOT NULL"`
	Order     int    `xorm:"NOT NULL DEFAULT 0"`
	CreatedAt int64  `xorm:"created"`
	UpdatedAt int64  `xorm:"updated"`
}

type KanbanCard struct {
	ID          int64  `xorm:"pk autoincr"`
	ColumnID    int64  `xorm:"INDEX NOT NULL"`
	IssueID     int64  `xorm:"INDEX"`
	Title       string `xorm:"NOT NULL"`
	Description string `xorm:"TEXT"`
	Order       int    `xorm:"NOT NULL DEFAULT 0"`
	CreatedAt   int64  `xorm:"created"`
	UpdatedAt   int64  `xorm:"updated"`
}
