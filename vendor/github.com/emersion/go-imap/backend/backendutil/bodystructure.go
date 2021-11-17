package backendutil

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"mime"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/textproto"
)

type countReader struct {
	r          io.Reader
	bytes      uint32
	newlines   uint32
	endsWithLF bool
}

func (r *countReader) Read(b []byte) (int, error) {
	n, err := r.r.Read(b)
	r.bytes += uint32(n)
	if n != 0 {
		r.newlines += uint32(bytes.Count(b[:n], []byte{'\n'}))
		r.endsWithLF = b[n-1] == '\n'
	}
	// If the stream does not end with a newline - count missing newline.
	if err == io.EOF {
		if !r.endsWithLF {
			r.newlines++
		}
	}
	return n, err
}

// FetchBodyStructure computes a message's body structure from its content.
func FetchBodyStructure(header textproto.Header, body io.Reader, extended bool) (*imap.BodyStructure, error) {
	bs := new(imap.BodyStructure)

	mediaType, mediaParams, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err == nil {
		typeParts := strings.SplitN(mediaType, "/", 2)
		bs.MIMEType = typeParts[0]
		if len(typeParts) == 2 {
			bs.MIMESubType = typeParts[1]
		}
		bs.Params = mediaParams
	} else {
		bs.MIMEType = "text"
		bs.MIMESubType = "plain"
	}

	bs.Id = header.Get("Content-Id")
	bs.Description = header.Get("Content-Description")
	bs.Encoding = header.Get("Content-Transfer-Encoding")

	if mr := multipartReader(header, body); mr != nil {
		var parts []*imap.BodyStructure
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}

			pbs, err := FetchBodyStructure(p.Header, p, extended)
			if err != nil {
				return nil, err
			}
			parts = append(parts, pbs)
		}
		bs.Parts = parts
	} else {
		countedBody := countReader{r: body}
		needLines := false
		if bs.MIMEType == "message" && bs.MIMESubType == "rfc822" {
			// This will result in double-buffering if body is already a
			// bufio.Reader (most likely it is). :\
			bufBody := bufio.NewReader(&countedBody)
			subMsgHdr, err := textproto.ReadHeader(bufBody)
			if err != nil {
				return nil, err
			}
			bs.Envelope, err = FetchEnvelope(subMsgHdr)
			if err != nil {
				return nil, err
			}
			bs.BodyStructure, err = FetchBodyStructure(subMsgHdr, bufBody, extended)
			if err != nil {
				return nil, err
			}
			needLines = true
		} else if bs.MIMEType == "text" {
			needLines = true
		}
		if _, err := io.Copy(ioutil.Discard, &countedBody); err != nil {
			return nil, err
		}
		bs.Size = countedBody.bytes
		if needLines {
			bs.Lines = countedBody.newlines
		}
	}

	if extended {
		bs.Extended = true
		bs.Disposition, bs.DispositionParams, _ = mime.ParseMediaType(header.Get("Content-Disposition"))

		// TODO: bs.Language, bs.Location
		// TODO: bs.MD5
	}

	return bs, nil
}
