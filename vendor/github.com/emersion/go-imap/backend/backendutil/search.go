package backendutil

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
	"github.com/emersion/go-message/textproto"
)

func matchString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func bufferBody(e *message.Entity) (*bytes.Buffer, error) {
	b := new(bytes.Buffer)
	if _, err := io.Copy(b, e.Body); err != nil {
		return nil, err
	}
	e.Body = b
	return b, nil
}

func matchBody(e *message.Entity, substr string) (bool, error) {
	if s, ok := e.Body.(fmt.Stringer); ok {
		return matchString(s.String(), substr), nil
	}

	b, err := bufferBody(e)
	if err != nil {
		return false, err
	}
	return matchString(b.String(), substr), nil
}

type lengther interface {
	Len() int
}

type countWriter struct {
	N int
}

func (w *countWriter) Write(b []byte) (int, error) {
	w.N += len(b)
	return len(b), nil
}

func bodyLen(e *message.Entity) (int, error) {
	headerSize := countWriter{}
	textproto.WriteHeader(&headerSize, e.Header.Header)

	if l, ok := e.Body.(lengther); ok {
		return l.Len() + headerSize.N, nil
	}

	b, err := bufferBody(e)
	if err != nil {
		return 0, err
	}
	return b.Len() + headerSize.N, nil
}

// Match returns true if a message and its metadata matches the provided
// criteria.
func Match(e *message.Entity, seqNum, uid uint32, date time.Time, flags []string, c *imap.SearchCriteria) (bool, error) {
	// TODO: support encoded header fields for Bcc, Cc, From, To
	// TODO: add header size for Larger and Smaller

	h := mail.Header{Header: e.Header}

	if !c.SentBefore.IsZero() || !c.SentSince.IsZero() {
		t, err := h.Date()
		if err != nil {
			return false, err
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)

		if !c.SentBefore.IsZero() && !t.Before(c.SentBefore) {
			return false, nil
		}
		if !c.SentSince.IsZero() && t.Before(c.SentSince) {
			return false, nil
		}
	}

	for key, wantValues := range c.Header {
		ok := e.Header.Has(key)
		for _, wantValue := range wantValues {
			if wantValue == "" && !ok {
				return false, nil
			}
			if wantValue != "" {
				ok := false
				values := e.Header.FieldsByKey(key)
				for values.Next() {
					decoded, _ := values.Text()
					if matchString(decoded, wantValue) {
						ok = true
						break
					}
				}
				if !ok {
					return false, nil
				}
			}
		}
	}
	for _, body := range c.Body {
		if ok, err := matchBody(e, body); err != nil || !ok {
			return false, err
		}
	}
	for _, text := range c.Text {
		headerMatch := false
		for f := e.Header.Fields(); f.Next(); {
			decoded, err := f.Text()
			if err != nil {
				continue
			}
			if strings.Contains(f.Key()+": "+decoded, text) {
				headerMatch = true
			}
		}
		if ok, err := matchBody(e, text); err != nil || !ok && !headerMatch {
			return false, err
		}
	}

	if c.Larger > 0 || c.Smaller > 0 {
		n, err := bodyLen(e)
		if err != nil {
			return false, err
		}

		if c.Larger > 0 && uint32(n) <= c.Larger {
			return false, nil
		}
		if c.Smaller > 0 && uint32(n) >= c.Smaller {
			return false, nil
		}
	}

	if !c.Since.IsZero() || !c.Before.IsZero() {
		if !matchDate(date, c) {
			return false, nil
		}
	}

	if c.WithFlags != nil || c.WithoutFlags != nil {
		if !matchFlags(flags, c) {
			return false, nil
		}
	}

	if c.SeqNum != nil || c.Uid != nil {
		if !matchSeqNumAndUid(seqNum, uid, c) {
			return false, nil
		}
	}

	for _, not := range c.Not {
		ok, err := Match(e, seqNum, uid, date, flags, not)
		if err != nil || ok {
			return false, err
		}
	}
	for _, or := range c.Or {
		ok1, err := Match(e, seqNum, uid, date, flags, or[0])
		if err != nil {
			return ok1, err
		}

		ok2, err := Match(e, seqNum, uid, date, flags, or[1])
		if err != nil || (!ok1 && !ok2) {
			return false, err
		}
	}

	return true, nil
}

func matchFlags(flags []string, c *imap.SearchCriteria) bool {
	flagsMap := make(map[string]bool)
	for _, f := range flags {
		flagsMap[f] = true
	}

	for _, f := range c.WithFlags {
		if !flagsMap[f] {
			return false
		}
	}
	for _, f := range c.WithoutFlags {
		if flagsMap[f] {
			return false
		}
	}

	return true
}

func matchSeqNumAndUid(seqNum uint32, uid uint32, c *imap.SearchCriteria) bool {
	if c.SeqNum != nil && !c.SeqNum.Contains(seqNum) {
		return false
	}
	if c.Uid != nil && !c.Uid.Contains(uid) {
		return false
	}
	return true
}

func matchDate(date time.Time, c *imap.SearchCriteria) bool {
	// We discard time zone information by setting it to UTC.
	// RFC 3501 explicitly requires zone unaware date comparison.
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	if !c.Since.IsZero() && !date.After(c.Since) {
		return false
	}
	if !c.Before.IsZero() && !date.Before(c.Before) {
		return false
	}
	return true
}
