package transfer

import (
	"fmt"
	"io"

	"github.com/git-lfs/pktline"
)

const (
	// Flush is the flush packet.
	Flush = '\x00'
	// Delim is the delimiter packet.
	Delim = '\x01'
)

// List of Git LFS commands.
const (
	versionCommand      = "version"
	batchCommand        = "batch"
	putObjectCommand    = "put-object"
	verifyObjectCommand = "verify-object"
	getObjectCommand    = "get-object"
	lockCommand         = "lock"
	listLockCommand     = "list-lock"
	unlockCommand       = "unlock"
	quitCommand         = "quit"
)

// PktLine is a Git packet line handler.
type Pktline struct {
	*pktline.Pktline
	r      io.Reader
	w      io.Writer
	logger Logger
}

// NewPktline creates a new Git packet line handler.
func NewPktline(r io.Reader, w io.Writer, logger Logger) *Pktline {
	if logger == nil {
		logger = new(noopLogger)
	}
	return &Pktline{
		Pktline: pktline.NewPktline(r, w),
		r:       r,
		w:       w,
		logger:  logger,
	}
}

// SendError sends an error msg.
func (p *Pktline) SendError(status uint32, message string) error {
	p.logger.Log("sending error status", "code", status, "msg", message)
	if err := p.WritePacketText(fmt.Sprintf("status %03d", status)); err != nil {
		p.logger.Log("failed to write packet", "err", err)
	}
	if err := p.WriteDelim(); err != nil {
		p.logger.Log("failed to write delimiter", "err", err)
	}
	if message != "" {
		if err := p.WritePacketText(message); err != nil {
			p.logger.Log("failed to write message", "err", err)
		}
	}
	return p.WriteFlush()
}

// SendStatus sends a status message.
func (p *Pktline) SendStatus(status Status) error {
	p.logger.Log("sending status", "code", status)
	if err := p.WritePacketText(fmt.Sprintf("status %03d", status.Code())); err != nil {
		p.logger.Log("failed to write status", "err", err)
	}
	if args := status.Args(); len(args) > 0 {
		for _, arg := range args {
			if err := p.WritePacketText(arg); err != nil {
				p.logger.Log("failed to write argument", "arg", arg, "err", err)
			}
		}
	}
	if msgs := status.Messages(); msgs != nil {
		if err := p.WriteDelim(); err != nil {
			p.logger.Log("failed to write delimiter", "err", err)
		}
		for _, msg := range msgs {
			if err := p.WritePacketText(msg); err != nil {
				p.logger.Log("failed to write message", "err", err)
			}
		}
	} else if r := status.Reader(); r != nil {
		p.logger.Log("sending reader")
		// Close reader if it implements io.Closer.
		if c, ok := r.(io.Closer); ok {
			defer c.Close()
		}
		if err := p.WriteDelim(); err != nil {
			p.logger.Log("failed to write delimiter", "err", err)
		}
		w := p.Writer()
		if _, err := io.Copy(w, r); err != nil {
			p.logger.Log("failed to copy reader", "err", err)
		}
		defer p.logger.Log("done copying")
		return w.Flush()
	}
	return p.WriteFlush()
}

// Reader returns a reader for the packet line.
func (p *Pktline) Reader() *pktline.PktlineReader {
	return p.ReaderWithSize(pktline.MaxPacketLength)
}

// ReaderWithSize returns a reader for the packet line with the given size.
func (p *Pktline) ReaderWithSize(size int) *pktline.PktlineReader {
	return pktline.NewPktlineReaderFromPktline(p.Pktline, size)
}

// Writer returns a writer for the packet line.
func (p *Pktline) Writer() *pktline.PktlineWriter {
	return p.WriterWithSize(pktline.MaxPacketLength)
}

// WriterWithSize returns a writer for the packet line with the given size.
func (p *Pktline) WriterWithSize(size int) *pktline.PktlineWriter {
	return pktline.NewPktlineWriterFromPktline(p.Pktline, size)
}

// ReadPacketListToDelim reads as many packets as possible using the `readPacketText`
// function before encountering a delim packet. It returns a slice of all the
// packets it read, or an error if one was encountered.
func (p *Pktline) ReadPacketListToDelim() ([]string, error) {
	var list []string
	for {
		data, pktLen, err := p.ReadPacketTextWithLength()
		if err != nil {
			return nil, err
		}

		if pktLen == Delim {
			break
		}

		list = append(list, data)
	}

	return list, nil
}

// ReadPacketListToFlush reads as many packets as possible using the `readPacketText`
// function before encountering a flush packet. It returns a slice of all the
// packets it read, or an error if one was encountered.
func (p *Pktline) ReadPacketListToFlush() ([]string, error) {
	var list []string
	for {
		data, pktLen, err := p.ReadPacketTextWithLength()
		if err != nil {
			return nil, err
		}

		if pktLen == Flush {
			break
		}

		list = append(list, data)
	}

	return list, nil
}
