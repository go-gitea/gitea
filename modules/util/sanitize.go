// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"bytes"
	"net"
	"strings"
)

type sanitizedError struct {
	err error
}

func (err sanitizedError) Error() string {
	return SanitizeCredentialURLs(err.err.Error())
}

func (err sanitizedError) Unwrap() error {
	return err.err
}

// SanitizeErrorCredentialURLs wraps the error and make sure the returned error message doesn't contain sensitive credentials in URLs
func SanitizeErrorCredentialURLs(err error) error {
	return sanitizedError{err: err}
}

var schemeSep = []byte("://")

const userInfoPlaceholder = "(masked)"

// SanitizeCredentialURLs remove all credentials in URLs for the input string:
// * "https://userinfo@domain.com" => "https://***@domain.com"
// * "user:pass@domain.com" => "***@domain.com"
// "***" is a magic string internally used, doesn't guarantee to be anything.
func SanitizeCredentialURLs(s string) string {
	sepColPos := strings.Index(s, ":")
	if sepColPos == -1 {
		return s // fast path: no colon, unlikely contain any URL credential
	}
	sepAtPos := strings.Index(s[sepColPos+1:], "@")
	for sepAtPos == -1 {
		return s // fast path: no "@" after colon, unlikely contain any URL credential
	}
	sepAtPos += sepColPos + 1

	res := make([]byte, 0, len(s)+len(userInfoPlaceholder)) // a best guess to avoid too many re-allocations
	bs := UnsafeStringToBytes(s)
	for {
		// left part (before "@") is likely to be the "userinfo" (single username, or "username:password")
		leftPos := sepAtPos - 1
	leftLoop:
		for leftPos >= 0 {
			c := bs[leftPos]
			switch c {
			case '-', '.', '_', '~', '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', ':', '%':
				// RFC 3986, userinfo can contain - . _ ~ ! $ & ' ( ) * + , ; = : and any percent-encoded chars
			default:
				valid := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
				if !valid {
					break leftLoop
				}
			}
			leftPos--
		}
		// left pos should point to the beginning of the left part, this pos is always valid in the buffer
		leftPos++

		// right part is likely to be the host (domain name, ip address)
		rightPos := sepAtPos + 1
	rightLoop:
		for rightPos < len(bs) {
			c := bs[rightPos]
			switch c {
			case '.', '-':
				// valid host char
			case '[':
				// ipv6 begin
				if rightPos != sepAtPos+1 {
					break rightLoop
				}
			case ']':
				// ipv6 end
				rightPos++
				break rightLoop
			default:
				valid := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
				if bs[sepAtPos+1] == '[' {
					// ipv6 host
					valid = 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F' || '0' <= c && c <= '9' || c == ':'
				}
				if !valid {
					break rightLoop
				}
			}
			rightPos++
		}

		leading, leftPart, rightPart := bs[:leftPos], bs[leftPos:sepAtPos], bs[sepAtPos+1:rightPos]

		// Either:
		// * git log message: "user:pass@host" (it contains a colon in userinfo), ignore "git@host" pattern
		// * http like URL: "https://userinfo@host.com" (it has "://" before the userinfo)
		needSanitize := bytes.IndexByte(leftPart, ':') >= 0 || bytes.HasSuffix(leading, schemeSep)
		needSanitize = needSanitize && len(leftPart) > 0 && len(rightPart) > 0
		// TODO: can also do more checks for right part
		// for example: ipv6 quick check
		if needSanitize && rightPart[0] == '[' {
			needSanitize = rightPart[len(rightPart)-1] == ']' && net.ParseIP(UnsafeBytesToString(rightPart[1:len(rightPart)-1])) != nil
		}
		if needSanitize {
			res = append(res, leading...)
			res = append(res, userInfoPlaceholder...)
			res = append(res, '@')
			res = append(res, rightPart...)
		} else {
			res = append(res, bs[:rightPos]...)
		}
		bs = bs[rightPos:]
		sepAtPos = bytes.IndexByte(bs, '@')
		if sepAtPos == -1 {
			break
		}
	}
	res = append(res, bs...)
	return UnsafeBytesToString(res)
}
