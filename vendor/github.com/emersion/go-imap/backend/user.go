package backend

import "errors"

var (
	// ErrNoSuchMailbox is returned by User.GetMailbox, User.DeleteMailbox and
	// User.RenameMailbox when retrieving, deleting or renaming a mailbox that
	// doesn't exist.
	ErrNoSuchMailbox = errors.New("No such mailbox")
	// ErrMailboxAlreadyExists is returned by User.CreateMailbox and
	// User.RenameMailbox when creating or renaming mailbox that already exists.
	ErrMailboxAlreadyExists = errors.New("Mailbox already exists")
)

// User represents a user in the mail storage system. A user operation always
// deals with mailboxes.
type User interface {
	// Username returns this user's username.
	Username() string

	// ListMailboxes returns a list of mailboxes belonging to this user. If
	// subscribed is set to true, only returns subscribed mailboxes.
	ListMailboxes(subscribed bool) ([]Mailbox, error)

	// GetMailbox returns a mailbox. If it doesn't exist, it returns
	// ErrNoSuchMailbox.
	GetMailbox(name string) (Mailbox, error)

	// CreateMailbox creates a new mailbox.
	//
	// If the mailbox already exists, an error must be returned. If the mailbox
	// name is suffixed with the server's hierarchy separator character, this is a
	// declaration that the client intends to create mailbox names under this name
	// in the hierarchy.
	//
	// If the server's hierarchy separator character appears elsewhere in the
	// name, the server SHOULD create any superior hierarchical names that are
	// needed for the CREATE command to be successfully completed.  In other
	// words, an attempt to create "foo/bar/zap" on a server in which "/" is the
	// hierarchy separator character SHOULD create foo/ and foo/bar/ if they do
	// not already exist.
	//
	// If a new mailbox is created with the same name as a mailbox which was
	// deleted, its unique identifiers MUST be greater than any unique identifiers
	// used in the previous incarnation of the mailbox UNLESS the new incarnation
	// has a different unique identifier validity value.
	CreateMailbox(name string) error

	// DeleteMailbox permanently remove the mailbox with the given name. It is an
	// error to // attempt to delete INBOX or a mailbox name that does not exist.
	//
	// The DELETE command MUST NOT remove inferior hierarchical names. For
	// example, if a mailbox "foo" has an inferior "foo.bar" (assuming "." is the
	// hierarchy delimiter character), removing "foo" MUST NOT remove "foo.bar".
	//
	// The value of the highest-used unique identifier of the deleted mailbox MUST
	// be preserved so that a new mailbox created with the same name will not
	// reuse the identifiers of the former incarnation, UNLESS the new incarnation
	// has a different unique identifier validity value.
	DeleteMailbox(name string) error

	// RenameMailbox changes the name of a mailbox. It is an error to attempt to
	// rename from a mailbox name that does not exist or to a mailbox name that
	// already exists.
	//
	// If the name has inferior hierarchical names, then the inferior hierarchical
	// names MUST also be renamed.  For example, a rename of "foo" to "zap" will
	// rename "foo/bar" (assuming "/" is the hierarchy delimiter character) to
	// "zap/bar".
	//
	// If the server's hierarchy separator character appears in the name, the
	// server SHOULD create any superior hierarchical names that are needed for
	// the RENAME command to complete successfully.  In other words, an attempt to
	// rename "foo/bar/zap" to baz/rag/zowie on a server in which "/" is the
	// hierarchy separator character SHOULD create baz/ and baz/rag/ if they do
	// not already exist.
	//
	// The value of the highest-used unique identifier of the old mailbox name
	// MUST be preserved so that a new mailbox created with the same name will not
	// reuse the identifiers of the former incarnation, UNLESS the new incarnation
	// has a different unique identifier validity value.
	//
	// Renaming INBOX is permitted, and has special behavior.  It moves all
	// messages in INBOX to a new mailbox with the given name, leaving INBOX
	// empty.  If the server implementation supports inferior hierarchical names
	// of INBOX, these are unaffected by a rename of INBOX.
	RenameMailbox(existingName, newName string) error

	// Logout is called when this User will no longer be used, likely because the
	// client closed the connection.
	Logout() error
}
