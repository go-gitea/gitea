package backendutil

import (
	"bytes"
	"errors"
	"io"
	"mime"
	nettextproto "net/textproto"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/textproto"
)

var errNoSuchPart = errors.New("backendutil: no such message body part")

func multipartReader(header textproto.Header, body io.Reader) *textproto.MultipartReader {
	contentType := header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return nil
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil
	}

	return textproto.NewMultipartReader(body, params["boundary"])
}

// FetchBodySection extracts a body section from a message.
func FetchBodySection(header textproto.Header, body io.Reader, section *imap.BodySectionName) (imap.Literal, error) {
	// First, find the requested part using the provided path
	for i := 0; i < len(section.Path); i++ {
		n := section.Path[i]

		mr := multipartReader(header, body)
		if mr == nil {
			// First part of non-multipart message refers to the message itself.
			// See RFC 3501, Page 55.
			if len(section.Path) == 1 && section.Path[0] == 1 {
				break
			}
			return nil, errNoSuchPart
		}

		for j := 1; j <= n; j++ {
			p, err := mr.NextPart()
			if err == io.EOF {
				return nil, errNoSuchPart
			} else if err != nil {
				return nil, err
			}

			if j == n {
				body = p
				header = p.Header

				break
			}
		}
	}

	// Then, write the requested data to a buffer
	b := new(bytes.Buffer)

	resHeader := header
	if section.Fields != nil {
		// Copy header so we will not change value passed to us.
		resHeader = header.Copy()

		if section.NotFields {
			for _, fieldName := range section.Fields {
				resHeader.Del(fieldName)
			}
		} else {
			fieldsMap := make(map[string]struct{}, len(section.Fields))
			for _, field := range section.Fields {
				fieldsMap[nettextproto.CanonicalMIMEHeaderKey(field)] = struct{}{}
			}

			for field := resHeader.Fields(); field.Next(); {
				if _, ok := fieldsMap[field.Key()]; !ok {
					field.Del()
				}
			}
		}
	}

	// Write the header
	err := textproto.WriteHeader(b, resHeader)
	if err != nil {
		return nil, err
	}

	switch section.Specifier {
	case imap.TextSpecifier:
		// The header hasn't been requested. Discard it.
		b.Reset()
	case imap.EntireSpecifier:
		if len(section.Path) > 0 {
			// When selecting a specific part by index, IMAP servers
			// return only the text, not the associated MIME header.
			b.Reset()
		}
	}

	// Write the body, if requested
	switch section.Specifier {
	case imap.EntireSpecifier, imap.TextSpecifier:
		if _, err := io.Copy(b, body); err != nil {
			return nil, err
		}
	}

	var l imap.Literal = b
	if section.Partial != nil {
		l = bytes.NewReader(section.ExtractPartial(b.Bytes()))
	}
	return l, nil
}
