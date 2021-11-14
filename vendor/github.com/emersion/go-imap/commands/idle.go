package commands

import (
	"github.com/emersion/go-imap"
)

// An IDLE command.
// Se RFC 2177 section 3.
type Idle struct{}

func (cmd *Idle) Command() *imap.Command {
	return &imap.Command{Name: "IDLE"}
}

func (cmd *Idle) Parse(fields []interface{}) error {
	return nil
}
