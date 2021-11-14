package mail

import (
	"mime"
	"net/mail"
	"strings"

	"github.com/emersion/go-message"
)

// Address represents a single mail address.
// The type alias ensures that a net/mail.Address can be used wherever an
// Address is expected
type Address = mail.Address

func formatAddressList(l []*Address) string {
	formatted := make([]string, len(l))
	for i, a := range l {
		formatted[i] = a.String()
	}
	return strings.Join(formatted, ", ")
}

// ParseAddress parses a single RFC 5322 address, e.g. "Barry Gibbs <bg@example.com>"
// Use this function only if you parse from a string, if you have a Header use
// Header.AddressList instead
func ParseAddress(address string) (*Address, error) {
	parser := mail.AddressParser{
		&mime.WordDecoder{message.CharsetReader},
	}
	return parser.Parse(address)
}

// ParseAddressList parses the given string as a list of addresses.
// Use this function only if you parse from a string, if you have a Header use
// Header.AddressList instead
func ParseAddressList(list string) ([]*Address, error) {
	parser := mail.AddressParser{
		&mime.WordDecoder{message.CharsetReader},
	}
	return parser.ParseList(list)
}
