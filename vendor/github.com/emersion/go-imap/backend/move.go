package backend

import (
	"github.com/emersion/go-imap"
)

// MoveMailbox is a mailbox that supports moving messages.
type MoveMailbox interface {
	Mailbox

	// Move the specified message(s) to the end of the specified destination
	// mailbox. This means that a new message is created in the target mailbox
	// with a new UID, the original message is removed from the source mailbox,
	// and it appears to the client as a single action.
	//
	// If the destination mailbox does not exist, a server SHOULD return an error.
	// It SHOULD NOT automatically create the mailbox.
	MoveMessages(uid bool, seqset *imap.SeqSet, dest string) error
}
