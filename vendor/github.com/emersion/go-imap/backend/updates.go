package backend

import (
	"github.com/emersion/go-imap"
)

// Update contains user and mailbox information about an unilateral backend
// update.
type Update interface {
	// The user targeted by this update. If empty, all connected users will
	// be notified.
	Username() string
	// The mailbox targeted by this update. If empty, the update targets all
	// mailboxes.
	Mailbox() string
	// Done returns a channel that is closed when the update has been broadcast to
	// all clients.
	Done() chan struct{}
}

// NewUpdate creates a new update.
func NewUpdate(username, mailbox string) Update {
	return &update{
		username: username,
		mailbox:  mailbox,
	}
}

type update struct {
	username string
	mailbox  string
	done     chan struct{}
}

func (u *update) Username() string {
	return u.username
}

func (u *update) Mailbox() string {
	return u.mailbox
}

func (u *update) Done() chan struct{} {
	if u.done == nil {
		u.done = make(chan struct{})
	}
	return u.done
}

// StatusUpdate is a status update. See RFC 3501 section 7.1 for a list of
// status responses.
type StatusUpdate struct {
	Update
	*imap.StatusResp
}

// MailboxUpdate is a mailbox update.
type MailboxUpdate struct {
	Update
	*imap.MailboxStatus
}

// MailboxInfoUpdate is a maiblox info update.
type MailboxInfoUpdate struct {
	Update
	*imap.MailboxInfo
}

// MessageUpdate is a message update.
type MessageUpdate struct {
	Update
	*imap.Message
}

// ExpungeUpdate is an expunge update.
type ExpungeUpdate struct {
	Update
	SeqNum uint32
}

// BackendUpdater is a Backend that implements Updater is able to send
// unilateral backend updates. Backends not implementing this interface don't
// correctly send unilateral updates, for instance if a user logs in from two
// connections and deletes a message from one of them, the over is not aware
// that such a mesage has been deleted. More importantly, backends implementing
// Updater can notify the user for external updates such as new message
// notifications.
type BackendUpdater interface {
	// Updates returns a set of channels where updates are sent to.
	Updates() <-chan Update
}

// MailboxPoller is a Mailbox that is able to poll updates for new messages or
// message status updates during a period of inactivity.
type MailboxPoller interface {
	// Poll requests mailbox updates.
	Poll() error
}
