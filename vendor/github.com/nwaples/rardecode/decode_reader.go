package rardecode

import (
	"errors"
	"io"
)

const (
	minWindowSize    = 0x40000
	maxQueuedFilters = 8192
)

var (
	errTooManyFilters = errors.New("rardecode: too many filters")
	errInvalidFilter  = errors.New("rardecode: invalid filter")
)

// filter functions take a byte slice, the current output offset and
// returns transformed data.
type filter func(b []byte, offset int64) ([]byte, error)

// filterBlock is a block of data to be processed by a filter.
type filterBlock struct {
	length int    // length of block
	offset int    // bytes to be read before start of block
	reset  bool   // drop all existing queued filters
	filter filter // filter function
}

// decoder is the interface for decoding compressed data
type decoder interface {
	init(r io.ByteReader, reset bool) error // initialize decoder for current file
	fill(w *window) ([]*filterBlock, error) // fill window with decoded data, returning any filters
}

// window is a sliding window buffer.
type window struct {
	buf  []byte
	mask int // buf length mask
	r    int // index in buf for reads (beginning)
	w    int // index in buf for writes (end)
	l    int // length of bytes to be processed by copyBytes
	o    int // offset of bytes to be processed by copyBytes
}

// buffered returns the number of bytes yet to be read from window
func (w *window) buffered() int { return (w.w - w.r) & w.mask }

// available returns the number of bytes that can be written before the window is full
func (w *window) available() int { return (w.r - w.w - 1) & w.mask }

func (w *window) reset(log2size uint, clear bool) {
	size := 1 << log2size
	if size < minWindowSize {
		size = minWindowSize
	}
	if size > len(w.buf) {
		b := make([]byte, size)
		if clear {
			w.w = 0
		} else if len(w.buf) > 0 {
			n := copy(b, w.buf[w.w:])
			n += copy(b[n:], w.buf[:w.w])
			w.w = n
		}
		w.buf = b
		w.mask = size - 1
	} else if clear {
		for i := range w.buf {
			w.buf[i] = 0
		}
		w.w = 0
	}
	w.r = w.w
}

// writeByte writes c to the end of the window
func (w *window) writeByte(c byte) {
	w.buf[w.w] = c
	w.w = (w.w + 1) & w.mask
}

// copyBytes copies len bytes at off distance from the end
// to the end of the window.
func (w *window) copyBytes(len, off int) {
	len &= w.mask

	n := w.available()
	if len > n {
		// if there is not enough space availaible we copy
		// as much as we can and save the offset and length
		// of the remaining data to be copied later.
		w.l = len - n
		w.o = off
		len = n
	}

	i := (w.w - off) & w.mask
	for ; len > 0; len-- {
		w.buf[w.w] = w.buf[i]
		w.w = (w.w + 1) & w.mask
		i = (i + 1) & w.mask
	}
}

// read reads bytes from the beginning of the window into p
func (w *window) read(p []byte) (n int) {
	if w.r > w.w {
		n = copy(p, w.buf[w.r:])
		w.r = (w.r + n) & w.mask
		p = p[n:]
	}
	if w.r < w.w {
		l := copy(p, w.buf[w.r:w.w])
		w.r += l
		n += l
	}
	if w.l > 0 && n > 0 {
		// if we have successfully read data, copy any
		// leftover data from a previous copyBytes.
		l := w.l
		w.l = 0
		w.copyBytes(l, w.o)
	}
	return n
}

// decodeReader implements io.Reader for decoding compressed data in RAR archives.
type decodeReader struct {
	win     window  // sliding window buffer used as decode dictionary
	dec     decoder // decoder being used to unpack file
	tot     int64   // total bytes read
	buf     []byte  // filter input/output buffer
	outbuf  []byte  // filter output not yet read
	err     error
	filters []*filterBlock // list of filterBlock's, each with offset relative to previous in list
}

func (d *decodeReader) init(r io.ByteReader, dec decoder, winsize uint, reset bool) error {
	if reset {
		d.filters = nil
	}
	d.err = nil
	d.outbuf = nil
	d.tot = 0
	d.win.reset(winsize, reset)
	d.dec = dec
	return d.dec.init(r, reset)
}

func (d *decodeReader) readErr() error {
	err := d.err
	d.err = nil
	return err
}

// queueFilter adds a filterBlock to the end decodeReader's filters.
func (d *decodeReader) queueFilter(f *filterBlock) error {
	if f.reset {
		d.filters = nil
	}
	if len(d.filters) >= maxQueuedFilters {
		return errTooManyFilters
	}
	// offset & length must be < window size
	f.offset &= d.win.mask
	f.length &= d.win.mask
	// make offset relative to previous filter in list
	for _, fb := range d.filters {
		if f.offset < fb.offset {
			// filter block must not start before previous filter
			return errInvalidFilter
		}
		f.offset -= fb.offset
	}
	d.filters = append(d.filters, f)
	return nil
}

// processFilters processes any filters valid at the current read index
// and stores the output in outbuf.
func (d *decodeReader) processFilters() (err error) {
	f := d.filters[0]
	if f.offset > 0 {
		return nil
	}
	d.filters = d.filters[1:]
	if d.win.buffered() < f.length {
		// fill() didn't return enough bytes
		err = d.readErr()
		if err == nil || err == io.EOF {
			return errInvalidFilter
		}
		return err
	}

	if cap(d.buf) < f.length {
		d.buf = make([]byte, f.length)
	}
	d.outbuf = d.buf[:f.length]
	n := d.win.read(d.outbuf)
	for {
		// run filter passing buffer and total bytes read so far
		d.outbuf, err = f.filter(d.outbuf, d.tot)
		if err != nil {
			return err
		}
		if cap(d.outbuf) > cap(d.buf) {
			// Filter returned a bigger buffer, save it for future filters.
			d.buf = d.outbuf
		}
		if len(d.filters) == 0 {
			return nil
		}
		f = d.filters[0]

		if f.offset != 0 {
			// next filter not at current offset
			f.offset -= n
			return nil
		}
		if f.length != len(d.outbuf) {
			return errInvalidFilter
		}
		d.filters = d.filters[1:]

		if cap(d.outbuf) < cap(d.buf) {
			// Filter returned a smaller buffer. Copy it back to the saved buffer
			// so the next filter can make use of the larger buffer if needed.
			d.outbuf = append(d.buf[:0], d.outbuf...)
		}
	}
}

// fill fills the decodeReader's window
func (d *decodeReader) fill() {
	if d.err != nil {
		return
	}
	var fl []*filterBlock
	fl, d.err = d.dec.fill(&d.win) // fill window using decoder
	for _, f := range fl {
		err := d.queueFilter(f)
		if err != nil {
			d.err = err
			return
		}
	}
}

// Read decodes data and stores it in p.
func (d *decodeReader) Read(p []byte) (n int, err error) {
	if len(d.outbuf) == 0 {
		// no filter output, see if we need to create more
		if d.win.buffered() == 0 {
			// fill empty window
			d.fill()
			if d.win.buffered() == 0 {
				return 0, d.readErr()
			}
		} else if len(d.filters) > 0 {
			f := d.filters[0]
			if f.offset == 0 && f.length > d.win.buffered() {
				d.fill() // filter at current offset needs more data
			}
		}
		if len(d.filters) > 0 {
			if err := d.processFilters(); err != nil {
				return 0, err
			}
		}
	}
	if len(d.outbuf) > 0 {
		// copy filter output into p
		n = copy(p, d.outbuf)
		d.outbuf = d.outbuf[n:]
	} else if len(d.filters) > 0 {
		f := d.filters[0]
		if f.offset < len(p) {
			// only read data up to beginning of next filter
			p = p[:f.offset]
		}
		n = d.win.read(p) // read directly from window
		f.offset -= n     // adjust first filter offset by bytes just read
	} else {
		n = d.win.read(p) // read directly from window
	}
	d.tot += int64(n)
	return n, nil
}
