package responses

import (
	"github.com/emersion/go-imap"
)

// An ENABLED response, defined in RFC 5161 section 3.2.
type Enabled struct {
	Caps []string
}

func (r *Enabled) Handle(resp imap.Resp) error {
	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != "ENABLED" {
		return ErrUnhandled
	}

	if caps, err := imap.ParseStringList(fields); err != nil {
		return err
	} else {
		r.Caps = append(r.Caps, caps...)
	}

	return nil
}

func (r *Enabled) WriteTo(w *imap.Writer) error {
	fields := []interface{}{imap.RawString("ENABLED")}
	for _, cap := range r.Caps {
		fields = append(fields, imap.RawString(cap))
	}
	return imap.NewUntaggedResp(fields).WriteTo(w)
}
