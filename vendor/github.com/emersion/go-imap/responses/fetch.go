package responses

import (
	"github.com/emersion/go-imap"
)

const fetchName = "FETCH"

// A FETCH response.
// See RFC 3501 section 7.4.2
type Fetch struct {
	Messages chan *imap.Message
	SeqSet   *imap.SeqSet
	Uid      bool
}

func (r *Fetch) Handle(resp imap.Resp) error {
	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != fetchName {
		return ErrUnhandled
	} else if len(fields) < 1 {
		return errNotEnoughFields
	}

	seqNum, err := imap.ParseNumber(fields[0])
	if err != nil {
		return err
	}

	msgFields, _ := fields[1].([]interface{})
	msg := &imap.Message{SeqNum: seqNum}
	if err := msg.Parse(msgFields); err != nil {
		return err
	}

	if r.Uid && msg.Uid == 0 {
		// we requested UIDs and got a message without one --> unilateral update --> ignore
		return ErrUnhandled
	}

	var num uint32
	if r.Uid {
		num = msg.Uid
	} else {
		num = seqNum
	}

	// Check whether we obtained a result we requested with our SeqSet
	// If the result is not contained in our SeqSet we have to handle an additional special case:
	// In case we requested UIDs with a dynamic sequence (i.e. * or n:*) and the maximum UID of the mailbox
	// is less then our n, the server will supply us with the max UID (cf. RFC 3501 §6.4.8 and §9 `seq-range`).
	// Thus, such a result is correct and has to be returned by us.
	if !r.SeqSet.Contains(num) && (!r.Uid || !r.SeqSet.Dynamic()) {
		return ErrUnhandled
	}

	r.Messages <- msg
	return nil
}

func (r *Fetch) WriteTo(w *imap.Writer) error {
	var err error
	for msg := range r.Messages {
		resp := imap.NewUntaggedResp([]interface{}{msg.SeqNum, imap.RawString(fetchName), msg.Format()})
		if err == nil {
			err = resp.WriteTo(w)
		}
	}
	return err
}
