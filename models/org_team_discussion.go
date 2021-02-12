// DNM-LEGAL(Krey): License

package models

type TeamDiscussion struct {
	ID               int64 `xorm:"pk autoincr"`
	PosterID         int64 `xorm:"INDEX"`
	Poster           *User `xorm:"-"`
	OriginalAuthor   string
	OriginalAuthorID int64  `xorm:"index"`
	Title            string `xorm:"name"`
	Content          string `xorm:"TEXT"`
	RenderedContent  string `xorm:"-"`
	NumComments      int
	Ref              string
	Attachments      []*Attachment `xorm:"-"`
	Comments         []*Comment    `xorm:"-"`
	Reactions        ReactionList  `xorm:"-"`
}

func init() {
	// DNM-TBD(Krey)
}
