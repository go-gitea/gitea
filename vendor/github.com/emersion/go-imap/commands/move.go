package commands

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// A MOVE command.
// See RFC 6851 section 3.1.
type Move struct {
	SeqSet  *imap.SeqSet
	Mailbox string
}

func (cmd *Move) Command() *imap.Command {
	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)

	return &imap.Command{
		Name:      "MOVE",
		Arguments: []interface{}{cmd.SeqSet, mailbox},
	}
}

func (cmd *Move) Parse(fields []interface{}) (err error) {
	if len(fields) < 2 {
		return errors.New("No enough arguments")
	}

	seqset, ok := fields[0].(string)
	if !ok {
		return errors.New("Invalid sequence set")
	}
	if cmd.SeqSet, err = imap.ParseSeqSet(seqset); err != nil {
		return err
	}

	mailbox, ok := fields[1].(string)
	if !ok {
		return errors.New("Mailbox name must be a string")
	}
	if cmd.Mailbox, err = utf7.Encoding.NewDecoder().String(mailbox); err != nil {
		return err
	}

	return
}
