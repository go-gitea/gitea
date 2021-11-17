package backend

import (
	"time"

	"github.com/emersion/go-imap"
)

// Mailbox represents a mailbox belonging to a user in the mail storage system.
// A mailbox operation always deals with messages.
type Mailbox interface {
	// Name returns this mailbox name.
	Name() string

	// Info returns this mailbox info.
	Info() (*imap.MailboxInfo, error)

	// Status returns this mailbox status. The fields Name, Flags, PermanentFlags
	// and UnseenSeqNum in the returned MailboxStatus must be always populated.
	// This function does not affect the state of any messages in the mailbox. See
	// RFC 3501 section 6.3.10 for a list of items that can be requested.
	Status(items []imap.StatusItem) (*imap.MailboxStatus, error)

	// SetSubscribed adds or removes the mailbox to the server's set of "active"
	// or "subscribed" mailboxes.
	SetSubscribed(subscribed bool) error

	// Check requests a checkpoint of the currently selected mailbox. A checkpoint
	// refers to any implementation-dependent housekeeping associated with the
	// mailbox (e.g., resolving the server's in-memory state of the mailbox with
	// the state on its disk). A checkpoint MAY take a non-instantaneous amount of
	// real time to complete. If a server implementation has no such housekeeping
	// considerations, CHECK is equivalent to NOOP.
	Check() error

	// ListMessages returns a list of messages. seqset must be interpreted as UIDs
	// if uid is set to true and as message sequence numbers otherwise. See RFC
	// 3501 section 6.4.5 for a list of items that can be requested.
	//
	// Messages must be sent to ch. When the function returns, ch must be closed.
	ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error

	// SearchMessages searches messages. The returned list must contain UIDs if
	// uid is set to true, or sequence numbers otherwise.
	SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error)

	// CreateMessage appends a new message to this mailbox. The \Recent flag will
	// be added no matter flags is empty or not. If date is nil, the current time
	// will be used.
	//
	// If the Backend implements Updater, it must notify the client immediately
	// via a mailbox update.
	CreateMessage(flags []string, date time.Time, body imap.Literal) error

	// UpdateMessagesFlags alters flags for the specified message(s).
	//
	// If the Backend implements Updater, it must notify the client immediately
	// via a message update.
	UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error

	// CopyMessages copies the specified message(s) to the end of the specified
	// destination mailbox. The flags and internal date of the message(s) SHOULD
	// be preserved, and the Recent flag SHOULD be set, in the copy.
	//
	// If the destination mailbox does not exist, a server SHOULD return an error.
	// It SHOULD NOT automatically create the mailbox.
	//
	// If the Backend implements Updater, it must notify the client immediately
	// via a mailbox update.
	CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error

	// Expunge permanently removes all messages that have the \Deleted flag set
	// from the currently selected mailbox.
	//
	// If the Backend implements Updater, it must notify the client immediately
	// via an expunge update.
	Expunge() error
}
