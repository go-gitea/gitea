package responses

import (
	"github.com/emersion/go-imap"
)

// An IDLE response.
type Idle struct {
	RepliesCh chan []byte
	Stop      <-chan struct{}

	gotContinuationReq bool
}

func (r *Idle) Replies() <-chan []byte {
	return r.RepliesCh
}

func (r *Idle) stop() {
	r.RepliesCh <- []byte("DONE\r\n")
}

func (r *Idle) Handle(resp imap.Resp) error {
	// Wait for a continuation request
	if _, ok := resp.(*imap.ContinuationReq); ok && !r.gotContinuationReq {
		r.gotContinuationReq = true

		// We got a continuation request, wait for r.Stop to be closed
		go func() {
			<-r.Stop
			r.stop()
		}()

		return nil
	}

	return ErrUnhandled
}
