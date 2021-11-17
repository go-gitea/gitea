// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/backend/backendutil"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/server"
)

var Delimiter = "/"

type testIMAPMailbox struct {
	Subscribed bool
	Messages   []*memory.Message

	Name_ string
	User  *testImapUser
}

func (mbox *testIMAPMailbox) Name() string {
	return mbox.Name_
}

func (mbox *testIMAPMailbox) Info() (*imap.MailboxInfo, error) {
	info := &imap.MailboxInfo{
		Delimiter: Delimiter,
		Name:      mbox.Name_,
	}
	return info, nil
}

func (mbox *testIMAPMailbox) uidNext() uint32 {
	var uid uint32
	for _, msg := range mbox.Messages {
		if msg.Uid > uid {
			uid = msg.Uid
		}
	}
	uid++
	return uid
}

func (mbox *testIMAPMailbox) flags() []string {
	flagsMap := make(map[string]bool)
	for _, msg := range mbox.Messages {
		for _, f := range msg.Flags {
			if !flagsMap[f] {
				flagsMap[f] = true
			}
		}
	}

	var flags []string
	for f := range flagsMap {
		flags = append(flags, f)
	}
	return flags
}

func (mbox *testIMAPMailbox) unseenSeqNum() uint32 {
	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		seen := false
		for _, flag := range msg.Flags {
			if flag == imap.SeenFlag {
				seen = true
				break
			}
		}

		if !seen {
			return seqNum
		}
	}
	return 0
}

func (mbox *testIMAPMailbox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {
	status := imap.NewMailboxStatus(mbox.Name_, items)
	status.Flags = mbox.flags()
	status.PermanentFlags = []string{"\\*"}
	status.UnseenSeqNum = mbox.unseenSeqNum()

	for _, name := range items {
		switch name {
		case imap.StatusMessages:
			status.Messages = uint32(len(mbox.Messages))
		case imap.StatusUidNext:
			status.UidNext = mbox.uidNext()
		case imap.StatusUidValidity:
			status.UidValidity = 1
		case imap.StatusRecent:
			status.Recent = 0
		case imap.StatusUnseen:
			status.Unseen = 0
		}
	}

	return status, nil
}

func (mbox *testIMAPMailbox) SetSubscribed(subscribed bool) error {
	mbox.Subscribed = subscribed
	return nil
}

func (mbox *testIMAPMailbox) Check() error {
	return nil
}

func (mbox *testIMAPMailbox) ListMessages(uid bool, seqSet *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {
	defer close(ch)

	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = seqNum
		}
		if !seqSet.Contains(id) {
			continue
		}

		m, err := msg.Fetch(seqNum, items)
		if err != nil {
			continue
		}

		ch <- m
	}

	return nil
}

func (mbox *testIMAPMailbox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	var ids []uint32
	for i, msg := range mbox.Messages {
		seqNum := uint32(i + 1)

		ok, err := msg.Match(seqNum, criteria)
		if err != nil || !ok {
			continue
		}

		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = seqNum
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (mbox *testIMAPMailbox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	if date.IsZero() {
		date = time.Now()
	}

	b, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	mbox.Messages = append(mbox.Messages, &memory.Message{
		Uid:   mbox.uidNext(),
		Date:  date,
		Size:  uint32(len(b)),
		Flags: flags,
		Body:  b,
	})
	return nil
}

func (mbox *testIMAPMailbox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, op imap.FlagsOp, flags []string) error {
	for i, msg := range mbox.Messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}

		msg.Flags = backendutil.UpdateFlags(msg.Flags, op, flags)
	}

	return nil
}

func (mbox *testIMAPMailbox) CopyMessages(uid bool, seqset *imap.SeqSet, destName string) error {
	dest, ok := mbox.User.Mailboxes[destName]
	if !ok {
		return backend.ErrNoSuchMailbox
	}

	for i, msg := range mbox.Messages {
		var id uint32
		if uid {
			id = msg.Uid
		} else {
			id = uint32(i + 1)
		}
		if !seqset.Contains(id) {
			continue
		}

		msgCopy := *msg
		msgCopy.Uid = dest.uidNext()
		dest.Messages = append(dest.Messages, &msgCopy)
	}

	return nil
}

func (mbox *testIMAPMailbox) Expunge() error {
	for i := len(mbox.Messages) - 1; i >= 0; i-- {
		msg := mbox.Messages[i]

		deleted := false
		for _, flag := range msg.Flags {
			if flag == imap.DeletedFlag {
				deleted = true
				break
			}
		}

		if deleted {
			mbox.Messages = append(mbox.Messages[:i], mbox.Messages[i+1:]...)
		}
	}

	return nil
}

type testImapUser struct {
	Name      string
	Password  string
	Mailboxes map[string]*testIMAPMailbox
}

func (u *testImapUser) Username() string {
	return u.Name
}

func (u *testImapUser) ListMailboxes(subscribed bool) (testIMAPMailboxes []backend.Mailbox, err error) {
	for _, testIMAPMailbox := range u.Mailboxes {
		if subscribed && !testIMAPMailbox.Subscribed {
			continue
		}

		testIMAPMailboxes = append(testIMAPMailboxes, testIMAPMailbox)
	}
	return
}

func (u *testImapUser) GetMailbox(name string) (testIMAPMailbox backend.Mailbox, err error) {
	testIMAPMailbox, ok := u.Mailboxes[name]
	if !ok {
		err = errors.New("no such Mailbox")
	}
	return
}

func (u *testImapUser) CreateMailbox(name string) error {
	return errors.New("TODO")
}

func (u *testImapUser) DeleteMailbox(name string) error {
	return errors.New("TODO")
}

func (u *testImapUser) RenameMailbox(existingName, newName string) error {
	return errors.New("TODO")
}

func (u *testImapUser) Logout() error {
	return nil
}

type testImapBacken struct {
	Users map[string]*testImapUser
}

func (b *testImapBacken) Login(connInfo *imap.ConnInfo, username, password string) (backend.User, error) {
	user, ok := b.Users[username]
	if ok && user.Password == password {
		return user, nil
	}

	return nil, errors.New("bad username or password")
}

func initTestBacken() *testImapBacken {
	body := "From: contact@example.org\r\n" +
		"To: contact@example.org\r\n" +
		"Subject: A little message, just for you\r\n" +
		"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
		"Message-ID: <0000000@localhost>\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Hi there :)"

	u := &testImapUser{
		Name:     "receive@gitea.io",
		Password: "123456",
	}

	u.Mailboxes = map[string]*testIMAPMailbox{
		"INBOX": {
			Name_: "INBOX",
			User:  u,
			Messages: []*memory.Message{
				{
					Uid:   6,
					Date:  time.Now(),
					Flags: []string{},
					Size:  uint32(len(body)),
					Body:  []byte(body),
				},
			},
		},
	}

	setting.MailRecieveService = &setting.MailReceiver{
		ReceiveEmail:   "receive@gitea.io",
		ReceiveBox:     "INBOX",
		QueueLength:    100,
		Host:           "127.0.0.1:1179",
		User:           "receive@gitea.io",
		Passwd:         "123456",
		IsTLSEnabled:   false,
		DeleteReadMail: false,
	}

	c = new(Client)

	c.UserName = setting.MailRecieveService.User
	c.Passwd = setting.MailRecieveService.Passwd
	c.Addr = setting.MailRecieveService.Host
	c.IsTLS = setting.MailRecieveService.IsTLSEnabled

	return &testImapBacken{
		Users: map[string]*testImapUser{
			u.Name: u,
		},
	}
}

func TestMain(m *testing.M) {
	s := server.New(initTestBacken())
	s.Addr = ":1179"
	// Since we will use this server for testing only, we can allow plain text
	// authentication over unencrypted connections
	s.AllowInsecureAuth = true

	end := make(chan bool, 1)
	go func() {
		_ = s.ListenAndServe()
		end <- true
	}()

	unittest.MainTest(m, filepath.Join("..", ".."))

	s.Close()
	<-end
}
