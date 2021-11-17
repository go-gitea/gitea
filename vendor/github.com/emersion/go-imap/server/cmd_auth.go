package server

import (
	"bufio"
	"errors"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-imap/responses"
)

// imap errors in Authenticated state.
var (
	ErrNotAuthenticated = errors.New("Not authenticated")
)

type Select struct {
	commands.Select
}

func (cmd *Select) Handle(conn Conn) error {
	ctx := conn.Context()

	// As per RFC1730#6.3.1,
	// 		The SELECT command automatically deselects any
	// 		currently selected mailbox before attempting the new selection.
	// 		Consequently, if a mailbox is selected and a SELECT command that
	// 		fails is attempted, no mailbox is selected.
	// For example, some clients (e.g. Apple Mail) perform SELECT "" when the
	// server doesn't announce the UNSELECT capability.
	ctx.Mailbox = nil
	ctx.MailboxReadOnly = false

	if ctx.User == nil {
		return ErrNotAuthenticated
	}
	mbox, err := ctx.User.GetMailbox(cmd.Mailbox)
	if err != nil {
		return err
	}

	items := []imap.StatusItem{
		imap.StatusMessages, imap.StatusRecent, imap.StatusUnseen,
		imap.StatusUidNext, imap.StatusUidValidity,
	}

	status, err := mbox.Status(items)
	if err != nil {
		return err
	}

	ctx.Mailbox = mbox
	ctx.MailboxReadOnly = cmd.ReadOnly || status.ReadOnly

	res := &responses.Select{Mailbox: status}
	if err := conn.WriteResp(res); err != nil {
		return err
	}

	var code imap.StatusRespCode = imap.CodeReadWrite
	if ctx.MailboxReadOnly {
		code = imap.CodeReadOnly
	}
	return ErrStatusResp(&imap.StatusResp{
		Type: imap.StatusRespOk,
		Code: code,
	})
}

type Create struct {
	commands.Create
}

func (cmd *Create) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	return ctx.User.CreateMailbox(cmd.Mailbox)
}

type Delete struct {
	commands.Delete
}

func (cmd *Delete) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	return ctx.User.DeleteMailbox(cmd.Mailbox)
}

type Rename struct {
	commands.Rename
}

func (cmd *Rename) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	return ctx.User.RenameMailbox(cmd.Existing, cmd.New)
}

type Subscribe struct {
	commands.Subscribe
}

func (cmd *Subscribe) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	mbox, err := ctx.User.GetMailbox(cmd.Mailbox)
	if err != nil {
		return err
	}

	return mbox.SetSubscribed(true)
}

type Unsubscribe struct {
	commands.Unsubscribe
}

func (cmd *Unsubscribe) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	mbox, err := ctx.User.GetMailbox(cmd.Mailbox)
	if err != nil {
		return err
	}

	return mbox.SetSubscribed(false)
}

type List struct {
	commands.List
}

func (cmd *List) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	ch := make(chan *imap.MailboxInfo)
	res := &responses.List{Mailboxes: ch, Subscribed: cmd.Subscribed}

	done := make(chan error, 1)
	go (func() {
		done <- conn.WriteResp(res)
		// Make sure to drain the channel.
		for range ch {
		}
	})()

	mailboxes, err := ctx.User.ListMailboxes(cmd.Subscribed)
	if err != nil {
		// Close channel to signal end of results
		close(ch)
		return err
	}

	for _, mbox := range mailboxes {
		info, err := mbox.Info()
		if err != nil {
			// Close channel to signal end of results
			close(ch)
			return err
		}

		// An empty ("" string) mailbox name argument is a special request to return
		// the hierarchy delimiter and the root name of the name given in the
		// reference.
		if cmd.Mailbox == "" {
			ch <- &imap.MailboxInfo{
				Attributes: []string{imap.NoSelectAttr},
				Delimiter:  info.Delimiter,
				Name:       info.Delimiter,
			}
			break
		}

		if info.Match(cmd.Reference, cmd.Mailbox) {
			ch <- info
		}
	}
	// Close channel to signal end of results
	close(ch)

	return <-done
}

type Status struct {
	commands.Status
}

func (cmd *Status) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	mbox, err := ctx.User.GetMailbox(cmd.Mailbox)
	if err != nil {
		return err
	}

	status, err := mbox.Status(cmd.Items)
	if err != nil {
		return err
	}

	// Only keep items thqat have been requested
	items := make(map[imap.StatusItem]interface{})
	for _, k := range cmd.Items {
		items[k] = status.Items[k]
	}
	status.Items = items

	res := &responses.Status{Mailbox: status}
	return conn.WriteResp(res)
}

type Append struct {
	commands.Append
}

func (cmd *Append) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.User == nil {
		return ErrNotAuthenticated
	}

	mbox, err := ctx.User.GetMailbox(cmd.Mailbox)
	if err == backend.ErrNoSuchMailbox {
		return ErrStatusResp(&imap.StatusResp{
			Type: imap.StatusRespNo,
			Code: imap.CodeTryCreate,
			Info: err.Error(),
		})
	} else if err != nil {
		return err
	}

	if err := mbox.CreateMessage(cmd.Flags, cmd.Date, cmd.Message); err != nil {
		if err == backend.ErrTooBig {
			return ErrStatusResp(&imap.StatusResp{
				Type: imap.StatusRespNo,
				Code: "TOOBIG",
				Info: "Message size exceeding limit",
			})
		}
		return err
	}

	// If APPEND targets the currently selected mailbox, send an untagged EXISTS
	// Do this only if the backend doesn't send updates itself
	if conn.Server().Updates == nil && ctx.Mailbox != nil && ctx.Mailbox.Name() == mbox.Name() {
		status, err := mbox.Status([]imap.StatusItem{imap.StatusMessages})
		if err != nil {
			return err
		}
		status.Flags = nil
		status.PermanentFlags = nil
		status.UnseenSeqNum = 0

		res := &responses.Select{Mailbox: status}
		if err := conn.WriteResp(res); err != nil {
			return err
		}
	}

	return nil
}

type Unselect struct {
	commands.Unselect
}

func (cmd *Unselect) Handle(conn Conn) error {
	ctx := conn.Context()
	if ctx.Mailbox == nil {
		return ErrNoMailboxSelected
	}

	ctx.Mailbox = nil
	ctx.MailboxReadOnly = false
	return nil
}

type Idle struct {
	commands.Idle
}

func (cmd *Idle) Handle(conn Conn) error {
	cont := &imap.ContinuationReq{Info: "idling"}
	if err := conn.WriteResp(cont); err != nil {
		return err
	}

	// Wait for DONE
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	if err := scanner.Err(); err != nil {
		return err
	}

	if strings.ToUpper(scanner.Text()) != "DONE" {
		return errors.New("Expected DONE")
	}
	return nil
}
