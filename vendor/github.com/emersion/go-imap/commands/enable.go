package commands

import (
	"github.com/emersion/go-imap"
)

// An ENABLE command, defined in RFC 5161 section 3.1.
type Enable struct {
	Caps []string
}

func (cmd *Enable) Command() *imap.Command {
	return &imap.Command{
		Name:      "ENABLE",
		Arguments: imap.FormatStringList(cmd.Caps),
	}
}

func (cmd *Enable) Parse(fields []interface{}) error {
	var err error
	cmd.Caps, err = imap.ParseStringList(fields)
	return err
}
