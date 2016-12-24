package iohelper

import "io"

type ByteMultiReader interface {
	io.ByteReader
	io.Reader
}
