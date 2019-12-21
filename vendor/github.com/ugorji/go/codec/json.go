// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

// By default, this json support uses base64 encoding for bytes, because you cannot
// store and read any arbitrary string in json (only unicode).
// However, the user can configre how to encode/decode bytes.
//
// This library specifically supports UTF-8 for encoding and decoding only.
//
// Note that the library will happily encode/decode things which are not valid
// json e.g. a map[int64]string. We do it for consistency. With valid json,
// we will encode and decode appropriately.
// Users can specify their map type if necessary to force it.
//
// Note:
//   - we cannot use strconv.Quote and strconv.Unquote because json quotes/unquotes differently.
//     We implement it here.

import (
	"bytes"
	"encoding/base64"
	"math"
	"strconv"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

//--------------------------------

var jsonLiterals = [...]byte{
	'"', 't', 'r', 'u', 'e', '"',
	'"', 'f', 'a', 'l', 's', 'e', '"',
	'"', 'n', 'u', 'l', 'l', '"',
}

const (
	jsonLitTrueQ  = 0
	jsonLitTrue   = 1
	jsonLitFalseQ = 6
	jsonLitFalse  = 7
	jsonLitNullQ  = 13
	jsonLitNull   = 14
)

var (
	// jsonLiteralTrueQ  = jsonLiterals[jsonLitTrueQ : jsonLitTrueQ+6]
	// jsonLiteralFalseQ = jsonLiterals[jsonLitFalseQ : jsonLitFalseQ+7]
	// jsonLiteralNullQ  = jsonLiterals[jsonLitNullQ : jsonLitNullQ+6]

	jsonLiteralTrue  = jsonLiterals[jsonLitTrue : jsonLitTrue+4]
	jsonLiteralFalse = jsonLiterals[jsonLitFalse : jsonLitFalse+5]
	jsonLiteralNull  = jsonLiterals[jsonLitNull : jsonLitNull+4]

	// these are used, after consuming the first char
	jsonLiteral4True  = jsonLiterals[jsonLitTrue+1 : jsonLitTrue+4]
	jsonLiteral4False = jsonLiterals[jsonLitFalse+1 : jsonLitFalse+5]
	jsonLiteral4Null  = jsonLiterals[jsonLitNull+1 : jsonLitNull+4]
)

const (
	jsonU4Chk2 = '0'
	jsonU4Chk1 = 'a' - 10
	jsonU4Chk0 = 'A' - 10

	// jsonScratchArrayLen = cacheLineSize + 32 // 96
)

const (
	// If !jsonValidateSymbols, decoding will be faster, by skipping some checks:
	//   - If we see first character of null, false or true,
	//     do not validate subsequent characters.
	//   - e.g. if we see a n, assume null and skip next 3 characters,
	//     and do not validate they are ull.
	// P.S. Do not expect a significant decoding boost from this.
	jsonValidateSymbols = true

	jsonSpacesOrTabsLen = 128

	jsonAlwaysReturnInternString = false
)

var (
	// jsonTabs and jsonSpaces are used as caches for indents
	jsonTabs, jsonSpaces [jsonSpacesOrTabsLen]byte

	jsonCharHtmlSafeSet   bitset256
	jsonCharSafeSet       bitset256
	jsonCharWhitespaceSet bitset256
	jsonNumSet            bitset256
)

func init() {
	var i byte
	for i = 0; i < jsonSpacesOrTabsLen; i++ {
		jsonSpaces[i] = ' '
		jsonTabs[i] = '\t'
	}

	// populate the safe values as true: note: ASCII control characters are (0-31)
	// jsonCharSafeSet:     all true except (0-31) " \
	// jsonCharHtmlSafeSet: all true except (0-31) " \ < > &
	for i = 32; i < utf8.RuneSelf; i++ {
		switch i {
		case '"', '\\':
		case '<', '>', '&':
			jsonCharSafeSet.set(i) // = true
		default:
			jsonCharSafeSet.set(i)
			jsonCharHtmlSafeSet.set(i)
		}
	}
	for i = 0; i <= utf8.RuneSelf; i++ {
		switch i {
		case ' ', '\t', '\r', '\n':
			jsonCharWhitespaceSet.set(i)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'e', 'E', '.', '+', '-':
			jsonNumSet.set(i)
		}
	}
}

// ----------------

type jsonEncDriver struct {
	noBuiltInTypes
	h *JsonHandle

	se interfaceExtWrapper

	// ---- cpu cache line boundary?
	di int8   // indent per: if negative, use tabs
	d  bool   // indenting?
	dl uint16 // indent level
	ks bool   // map key as string
	is byte   // integer as string

	typical bool

	s *bitset256 // safe set for characters (taking h.HTMLAsIs into consideration)
	// scratch: encode time, numbers, etc. Note: leave 1 byte for containerState
	b [cacheLineSize + 24]byte // buffer for encoding numbers and time

	e Encoder
}

// Keep writeIndent, WriteArrayElem, WriteMapElemKey, WriteMapElemValue
// in jsonEncDriver, so that *Encoder can directly call them

func (e *jsonEncDriver) encoder() *Encoder { return &e.e }

func (e *jsonEncDriver) writeIndent() {
	e.e.encWr.writen1('\n')
	x := int(e.di) * int(e.dl)
	if e.di < 0 {
		x = -x
		for x > jsonSpacesOrTabsLen {
			e.e.encWr.writeb(jsonTabs[:])
			x -= jsonSpacesOrTabsLen
		}
		e.e.encWr.writeb(jsonTabs[:x])
	} else {
		for x > jsonSpacesOrTabsLen {
			e.e.encWr.writeb(jsonSpaces[:])
			x -= jsonSpacesOrTabsLen
		}
		e.e.encWr.writeb(jsonSpaces[:x])
	}
}

func (e *jsonEncDriver) WriteArrayElem() {
	if e.e.c != containerArrayStart {
		e.e.encWr.writen1(',')
	}
	if e.d {
		e.writeIndent()
	}
}

func (e *jsonEncDriver) WriteMapElemKey() {
	if e.e.c != containerMapStart {
		e.e.encWr.writen1(',')
	}
	if e.d {
		e.writeIndent()
	}
}

func (e *jsonEncDriver) WriteMapElemValue() {
	if e.d {
		e.e.encWr.writen2(':', ' ')
	} else {
		e.e.encWr.writen1(':')
	}
}

func (e *jsonEncDriver) EncodeNil() {
	// We always encode nil as just null (never in quotes)
	// This allows us to easily decode if a nil in the json stream
	// ie if initial token is n.

	// e.e.encWr.writeb(jsonLiteralNull)
	e.e.encWr.writen([rwNLen]byte{'n', 'u', 'l', 'l'}, 4)
}

func (e *jsonEncDriver) EncodeTime(t time.Time) {
	// Do NOT use MarshalJSON, as it allocates internally.
	// instead, we call AppendFormat directly, using our scratch buffer (e.b)

	if t.IsZero() {
		e.EncodeNil()
	} else {
		e.b[0] = '"'
		b := fmtTime(t, e.b[1:1])
		e.b[len(b)+1] = '"'
		e.e.encWr.writeb(e.b[:len(b)+2])
	}
}

func (e *jsonEncDriver) EncodeExt(rv interface{}, xtag uint64, ext Ext) {
	if ext == SelfExt {
		rv2 := baseRV(rv)
		e.e.encodeValue(rv2, e.h.fnNoExt(rv2.Type()))
	} else if v := ext.ConvertExt(rv); v == nil {
		e.EncodeNil()
	} else {
		e.e.encode(v)
	}
}

func (e *jsonEncDriver) EncodeRawExt(re *RawExt) {
	// only encodes re.Value (never re.Data)
	if re.Value == nil {
		e.EncodeNil()
	} else {
		e.e.encode(re.Value)
	}
}

func (e *jsonEncDriver) EncodeBool(b bool) {
	// Use writen with an array instead of writeb with a slice
	// i.e. in place of e.e.encWr.writeb(jsonLiteralTrueQ)
	//      OR jsonLiteralTrue, jsonLiteralFalse, jsonLiteralFalseQ, etc

	if e.ks && e.e.c == containerMapKey {
		if b {
			e.e.encWr.writen([rwNLen]byte{'"', 't', 'r', 'u', 'e', '"'}, 6)
		} else {
			e.e.encWr.writen([rwNLen]byte{'"', 'f', 'a', 'l', 's', 'e', '"'}, 7)
		}
	} else {
		if b {
			e.e.encWr.writen([rwNLen]byte{'t', 'r', 'u', 'e'}, 4)
		} else {
			e.e.encWr.writen([rwNLen]byte{'f', 'a', 'l', 's', 'e'}, 5)
		}
	}
}

func (e *jsonEncDriver) encodeFloat(f float64, bitsize, fmt byte, prec int8) {
	var blen uint
	if e.ks && e.e.c == containerMapKey {
		blen = 2 + uint(len(strconv.AppendFloat(e.b[1:1], f, fmt, int(prec), int(bitsize))))
		// _ = e.b[:blen]
		e.b[0] = '"'
		e.b[blen-1] = '"'
		e.e.encWr.writeb(e.b[:blen])
	} else {
		e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], f, fmt, int(prec), int(bitsize)))
	}
}

func (e *jsonEncDriver) EncodeFloat64(f float64) {
	fmt, prec := jsonFloatStrconvFmtPrec64(f)
	e.encodeFloat(f, 64, fmt, prec)
}

func (e *jsonEncDriver) EncodeFloat32(f float32) {
	fmt, prec := jsonFloatStrconvFmtPrec32(f)
	e.encodeFloat(float64(f), 32, fmt, prec)
}

func (e *jsonEncDriver) EncodeInt(v int64) {
	if e.is == 'A' || e.is == 'L' && (v > 1<<53 || v < -(1<<53)) ||
		(e.ks && e.e.c == containerMapKey) {
		blen := 2 + len(strconv.AppendInt(e.b[1:1], v, 10))
		e.b[0] = '"'
		e.b[blen-1] = '"'
		e.e.encWr.writeb(e.b[:blen])
		return
	}
	e.e.encWr.writeb(strconv.AppendInt(e.b[:0], v, 10))
}

func (e *jsonEncDriver) EncodeUint(v uint64) {
	if e.is == 'A' || e.is == 'L' && v > 1<<53 || (e.ks && e.e.c == containerMapKey) {
		blen := 2 + len(strconv.AppendUint(e.b[1:1], v, 10))
		e.b[0] = '"'
		e.b[blen-1] = '"'
		e.e.encWr.writeb(e.b[:blen])
		return
	}
	e.e.encWr.writeb(strconv.AppendUint(e.b[:0], v, 10))
}

func (e *jsonEncDriver) EncodeString(v string) {
	if e.h.StringToRaw {
		e.EncodeStringBytesRaw(bytesView(v))
		return
	}
	e.quoteStr(v)
}

func (e *jsonEncDriver) EncodeStringBytesRaw(v []byte) {
	// if encoding raw bytes and RawBytesExt is configured, use it to encode
	if v == nil {
		e.EncodeNil()
		return
	}
	if e.se.InterfaceExt != nil {
		e.EncodeExt(v, 0, &e.se)
		return
	}

	slen := base64.StdEncoding.EncodedLen(len(v)) + 2
	var bs []byte
	if len(e.b) < slen {
		bs = e.e.blist.get(slen)
	} else {
		bs = e.b[:slen]
	}
	bs[0] = '"'
	base64.StdEncoding.Encode(bs[1:], v)
	bs[len(bs)-1] = '"'
	e.e.encWr.writeb(bs)
	if len(e.b) < slen {
		e.e.blist.put(bs)
	}
}

// indent is done as below:
//   - newline and indent are added before each mapKey or arrayElem
//   - newline and indent are added before each ending,
//     except there was no entry (so we can have {} or [])

func (e *jsonEncDriver) WriteArrayStart(length int) {
	if e.d {
		e.dl++
	}
	e.e.encWr.writen1('[')
}

func (e *jsonEncDriver) WriteArrayEnd() {
	if e.d {
		e.dl--
		e.writeIndent()
	}
	e.e.encWr.writen1(']')
}

func (e *jsonEncDriver) WriteMapStart(length int) {
	if e.d {
		e.dl++
	}
	e.e.encWr.writen1('{')
}

func (e *jsonEncDriver) WriteMapEnd() {
	if e.d {
		e.dl--
		if e.e.c != containerMapStart {
			e.writeIndent()
		}
	}
	e.e.encWr.writen1('}')
}

func (e *jsonEncDriver) quoteStr(s string) {
	// adapted from std pkg encoding/json
	const hex = "0123456789abcdef"
	w := e.e.w()
	w.writen1('"')
	var i, start uint
	for i < uint(len(s)) {
		// encode all bytes < 0x20 (except \r, \n).
		// also encode < > & to prevent security holes when served to some browsers.

		// We optimize for ascii, by assumining that most characters are in the BMP
		// and natively consumed by json without much computation.

		// if 0x20 <= b && b != '\\' && b != '"' && b != '<' && b != '>' && b != '&' {
		// if (htmlasis && jsonCharSafeSet.isset(b)) || jsonCharHtmlSafeSet.isset(b) {
		b := s[i]
		if e.s.isset(b) {
			i++
			continue
		}
		if b < utf8.RuneSelf {
			if start < i {
				w.writestr(s[start:i])
			}
			switch b {
			case '\\', '"':
				w.writen2('\\', b)
			case '\n':
				w.writen2('\\', 'n')
			case '\r':
				w.writen2('\\', 'r')
			case '\b':
				w.writen2('\\', 'b')
			case '\f':
				w.writen2('\\', 'f')
			case '\t':
				w.writen2('\\', 't')
			default:
				w.writestr(`\u00`)
				w.writen2(hex[b>>4], hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError {
			if size == 1 {
				if start < i {
					w.writestr(s[start:i])
				}
				w.writestr(`\ufffd`)
				i++
				start = i
			}
			continue
		}
		// U+2028 is LINE SEPARATOR. U+2029 is PARAGRAPH SEPARATOR.
		// Both technically valid JSON, but bomb on JSONP, so fix here unconditionally.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				w.writestr(s[start:i])
			}
			w.writestr(`\u202`)
			w.writen1(hex[c&0xF])
			i += uint(size)
			start = i
			continue
		}
		i += uint(size)
	}
	if start < uint(len(s)) {
		w.writestr(s[start:])
	}
	w.writen1('"')
}

func (e *jsonEncDriver) atEndOfEncode() {
	if e.h.TermWhitespace {
		if e.e.c == 0 { // scalar written, output space
			e.e.encWr.writen1(' ')
		} else { // container written, output new-line
			e.e.encWr.writen1('\n')
		}
	}
}

// ----------

type jsonDecDriver struct {
	noBuiltInTypes
	h *JsonHandle

	tok  uint8   // used to store the token read right after skipWhiteSpace
	fnil bool    // found null
	_    [2]byte // padding
	bstr [4]byte // scratch used for string \UXXX parsing

	buf []byte
	se  interfaceExtWrapper

	_ uint64 // padding

	// ---- cpu cache line boundary?

	d Decoder
}

// func jsonIsWS(b byte) bool {
// 	// return b == ' ' || b == '\t' || b == '\r' || b == '\n'
// 	return jsonCharWhitespaceSet.isset(b)
// }

func (d *jsonDecDriver) decoder() *Decoder {
	return &d.d
}

func (d *jsonDecDriver) uncacheRead() {
	if d.tok != 0 {
		d.d.decRd.unreadn1()
		d.tok = 0
	}
}

func (d *jsonDecDriver) ReadMapStart() int {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return decContainerLenNil
	}
	if d.tok != '{' {
		d.d.errorf("read map - expect char '%c' but got char '%c'", '{', d.tok)
	}
	d.tok = 0
	return decContainerLenUnknown
}

func (d *jsonDecDriver) ReadArrayStart() int {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return decContainerLenNil
	}
	if d.tok != '[' {
		d.d.errorf("read array - expect char '%c' but got char '%c'", '[', d.tok)
	}
	d.tok = 0
	return decContainerLenUnknown
}

func (d *jsonDecDriver) CheckBreak() bool {
	d.advance()
	return d.tok == '}' || d.tok == ']'
}

func (d *jsonDecDriver) ReadArrayElem() {
	const xc uint8 = ','
	if d.d.c != containerArrayStart {
		d.advance()
		if d.tok != xc {
			d.readDelimError(xc)
		}
		d.tok = 0
	}
}

func (d *jsonDecDriver) ReadArrayEnd() {
	const xc uint8 = ']'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

func (d *jsonDecDriver) ReadMapElemKey() {
	const xc uint8 = ','
	if d.d.c != containerMapStart {
		d.advance()
		if d.tok != xc {
			d.readDelimError(xc)
		}
		d.tok = 0
	}
}

func (d *jsonDecDriver) ReadMapElemValue() {
	const xc uint8 = ':'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

func (d *jsonDecDriver) ReadMapEnd() {
	const xc uint8 = '}'
	d.advance()
	if d.tok != xc {
		d.readDelimError(xc)
	}
	d.tok = 0
}

// func (d *jsonDecDriver) readDelim(xc uint8) {
// 	d.advance()
// 	if d.tok != xc {
// 		d.readDelimError(xc)
// 	}
// 	d.tok = 0
// }

func (d *jsonDecDriver) readDelimError(xc uint8) {
	d.d.errorf("read json delimiter - expect char '%c' but got char '%c'", xc, d.tok)
}

func (d *jsonDecDriver) readLit4True() {
	bs := d.d.decRd.readn(3)
	d.tok = 0
	if jsonValidateSymbols && bs != [rwNLen]byte{'r', 'u', 'e'} { // !Equal jsonLiteral4True
		d.d.errorf("expecting %s: got %s", jsonLiteral4True, bs)
	}
}

func (d *jsonDecDriver) readLit4False() {
	bs := d.d.decRd.readn(4)
	d.tok = 0
	if jsonValidateSymbols && bs != [rwNLen]byte{'a', 'l', 's', 'e'} { // !Equal jsonLiteral4False
		d.d.errorf("expecting %s: got %s", jsonLiteral4False, bs)
	}
}

func (d *jsonDecDriver) readLit4Null() {
	bs := d.d.decRd.readn(3) // readx(3)
	d.tok = 0
	if jsonValidateSymbols && bs != [rwNLen]byte{'u', 'l', 'l'} { // !Equal jsonLiteral4Null
		d.d.errorf("expecting %s: got %s", jsonLiteral4Null, bs)
	}
	d.fnil = true
}

func (d *jsonDecDriver) advance() {
	if d.tok == 0 {
		d.fnil = false
		d.tok = d.d.decRd.skip(&jsonCharWhitespaceSet)
	}
}

func (d *jsonDecDriver) TryNil() bool {
	d.advance()
	// we shouldn't try to see if quoted "null" was here, right?
	// only the plain string: `null` denotes a nil (ie not quotes)
	if d.tok == 'n' {
		d.readLit4Null()
		return true
	}
	return false
}

func (d *jsonDecDriver) Nil() bool {
	return d.fnil
}

func (d *jsonDecDriver) DecodeBool() (v bool) {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	fquot := d.d.c == containerMapKey && d.tok == '"'
	if fquot {
		d.tok = d.d.decRd.readn1()
	}
	switch d.tok {
	case 'f':
		d.readLit4False()
		// v = false
	case 't':
		d.readLit4True()
		v = true
	default:
		d.d.errorf("decode bool: got first char %c", d.tok)
		// v = false // "unreachable"
	}
	if fquot {
		d.d.decRd.readn1()
	}
	return
}

func (d *jsonDecDriver) DecodeTime() (t time.Time) {
	// read string, and pass the string into json.unmarshal
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	bs := d.readString()
	t, err := time.Parse(time.RFC3339, stringView(bs))
	if err != nil {
		d.d.errorv(err)
	}
	return
}

func (d *jsonDecDriver) ContainerType() (vt valueType) {
	// check container type by checking the first char
	d.advance()

	// optimize this, so we don't do 4 checks but do one computation.
	// return jsonContainerSet[d.tok]

	// ContainerType is mostly called for Map and Array,
	// so this conditional is good enough (max 2 checks typically)
	if d.tok == '{' {
		return valueTypeMap
	} else if d.tok == '[' {
		return valueTypeArray
	} else if d.tok == 'n' {
		d.readLit4Null()
		return valueTypeNil
	} else if d.tok == '"' {
		return valueTypeString
	}
	return valueTypeUnset
}

func (d *jsonDecDriver) decNumBytes() (bs []byte) {
	d.advance()
	if d.tok == '"' {
		bs = d.d.decRd.readUntil('"', false)
	} else if d.tok == 'n' {
		d.readLit4Null()
	} else {
		d.d.decRd.unreadn1()
		bs = d.d.decRd.readTo(&jsonNumSet)
	}
	d.tok = 0
	return
}

func (d *jsonDecDriver) DecodeUint64() (u uint64) {
	bs := d.decNumBytes()
	if len(bs) == 0 {
		return
	}
	n, neg, badsyntax, overflow := jsonParseInteger(bs)
	if overflow {
		d.d.errorf("overflow parsing unsigned integer: %s", bs)
	} else if neg {
		d.d.errorf("minus found parsing unsigned integer: %s", bs)
	} else if badsyntax {
		// fallback: try to decode as float, and cast
		n = d.decUint64ViaFloat(bs)
	}
	return n
}

func (d *jsonDecDriver) DecodeInt64() (i int64) {
	const cutoff = uint64(1 << uint(64-1))
	bs := d.decNumBytes()
	if len(bs) == 0 {
		return
	}
	n, neg, badsyntax, overflow := jsonParseInteger(bs)
	if overflow {
		d.d.errorf("overflow parsing integer: %s", bs)
	} else if badsyntax {
		// d.d.errorf("invalid syntax for integer: %s", bs)
		// fallback: try to decode as float, and cast
		if neg {
			n = d.decUint64ViaFloat(bs[1:])
		} else {
			n = d.decUint64ViaFloat(bs)
		}
	}
	if neg {
		if n > cutoff {
			d.d.errorf("overflow parsing integer: %s", bs)
		}
		i = -(int64(n))
	} else {
		if n >= cutoff {
			d.d.errorf("overflow parsing integer: %s", bs)
		}
		i = int64(n)
	}
	return
}

func (d *jsonDecDriver) decUint64ViaFloat(s []byte) (u uint64) {
	if len(s) == 0 {
		return
	}
	f, err := parseFloat64(s)
	if err != nil {
		d.d.errorf("invalid syntax for integer: %s", s)
	}
	fi, ff := math.Modf(f)
	if ff > 0 {
		d.d.errorf("fractional part found parsing integer: %s", s)
	} else if fi > float64(math.MaxUint64) {
		d.d.errorf("overflow parsing integer: %s", s)
	}
	return uint64(fi)
}

func (d *jsonDecDriver) DecodeFloat64() (f float64) {
	var err error
	if bs := d.decNumBytes(); len(bs) > 0 {
		if f, err = parseFloat64(bs); err != nil {
			d.d.errorv(err)
		}
	}
	return
}

func (d *jsonDecDriver) DecodeFloat32() (f float32) {
	var err error
	if bs := d.decNumBytes(); len(bs) > 0 {
		if f, err = parseFloat32(bs); err != nil {
			d.d.errorv(err)
		}
	}
	return
}

func (d *jsonDecDriver) DecodeExt(rv interface{}, xtag uint64, ext Ext) {
	d.advance()
	if d.tok == 'n' {
		d.readLit4Null()
		return
	}
	if ext == nil {
		re := rv.(*RawExt)
		re.Tag = xtag
		d.d.decode(&re.Value)
	} else if ext == SelfExt {
		rv2 := baseRV(rv)
		d.d.decodeValue(rv2, d.h.fnNoExt(rv2.Type()))
	} else {
		d.d.interfaceExtConvertAndDecode(rv, ext)
	}
}

func (d *jsonDecDriver) decBytesFromArray(bs []byte) []byte {
	if bs == nil {
		bs = []byte{}
	} else {
		bs = bs[:0]
	}
	d.tok = 0
	bs = append(bs, uint8(d.DecodeUint64()))
	d.tok = d.d.decRd.skip(&jsonCharWhitespaceSet)
	for d.tok != ']' {
		if d.tok != ',' {
			d.d.errorf("read array element - expect char '%c' but got char '%c'", ',', d.tok)
		}
		d.tok = 0
		bs = append(bs, uint8(chkOvf.UintV(d.DecodeUint64(), 8)))
		d.tok = d.d.decRd.skip(&jsonCharWhitespaceSet)
	}
	d.tok = 0
	return bs
}

func (d *jsonDecDriver) DecodeBytes(bs []byte, zerocopy bool) (bsOut []byte) {
	// if decoding into raw bytes, and the RawBytesExt is configured, use it to decode.
	if d.se.InterfaceExt != nil {
		bsOut = bs
		d.DecodeExt(&bsOut, 0, &d.se)
		return
	}
	d.advance()
	// check if an "array" of uint8's (see ContainerType for how to infer if an array)
	if d.tok == '[' {
		// bsOut, _ = fastpathTV.DecSliceUint8V(bs, true, d.d)
		if zerocopy && len(bs) == 0 {
			bs = d.d.b[:]
		}
		return d.decBytesFromArray(bs)
	}

	// base64 encodes []byte{} as "", and we encode nil []byte as null.
	// Consequently, base64 should decode null as a nil []byte, and "" as an empty []byte{}.
	// appendStringAsBytes returns a zero-len slice for both, so as not to reset d.buf.
	// However, it sets a fnil field to true, so we can check if a null was found.

	if d.tok == 'n' {
		d.readLit4Null()
		return nil
	}

	bs1 := d.readString()
	slen := base64.StdEncoding.DecodedLen(len(bs1))
	if slen == 0 {
		bsOut = []byte{}
	} else if slen <= cap(bs) {
		bsOut = bs[:slen]
	} else if zerocopy {
		d.buf = d.d.blist.check(d.buf, slen)
		bsOut = d.buf
	} else {
		bsOut = make([]byte, slen)
	}
	slen2, err := base64.StdEncoding.Decode(bsOut, bs1)
	if err != nil {
		d.d.errorf("error decoding base64 binary '%s': %v", bs1, err)
		return nil
	}
	if slen != slen2 {
		bsOut = bsOut[:slen2]
	}
	return
}

func (d *jsonDecDriver) DecodeStringAsBytes() (s []byte) {
	d.advance()
	if d.tok != '"' {
		// d.d.errorf("expect char '%c' but got char '%c'", '"', d.tok)
		// handle non-string scalar: null, true, false or a number
		switch d.tok {
		case 'n':
			d.readLit4Null()
			return []byte{}
		case 'f':
			d.readLit4False()
			return jsonLiteralFalse
		case 't':
			d.readLit4True()
			return jsonLiteralTrue
		}
		// try to parse a valid number
		return d.decNumBytes()
	}
	s = d.appendStringAsBytes()
	if d.fnil {
		return nil
	}
	return
}

func (d *jsonDecDriver) readString() (bs []byte) {
	if d.tok != '"' {
		d.d.errorf("expecting string starting with '\"'; got '%c'", d.tok)
		return
	}

	bs = d.d.decRd.readUntil('"', false)
	d.tok = 0
	return
}

func (d *jsonDecDriver) appendStringAsBytes() (bs []byte) {
	if d.buf != nil {
		d.buf = d.buf[:0]
	}
	d.tok = 0

	// append on each byte seen can be expensive, so we just
	// keep track of where we last read a contiguous set of
	// non-special bytes (using cursor variable),
	// and when we see a special byte
	// e.g. end-of-slice, " or \,
	// we will append the full range into the v slice before proceeding

	var cs = d.d.decRd.readUntil('"', true)
	var c uint8
	var i, cursor uint
	for {
		if i >= uint(len(cs)) {
			d.buf = append(d.buf, cs[cursor:]...)
			cs = d.d.decRd.readUntil('"', true)
			i, cursor = 0, 0
			continue // this continue helps elide the cs[i] below
		}
		c = cs[i]
		if c == '"' {
			break
		}
		if c != '\\' {
			i++
			continue
		}

		d.buf = append(d.buf, cs[cursor:i]...)
		i++
		if i >= uint(len(cs)) {
			d.d.errorf("need at least 1 more bytes for \\ escape sequence")
			return // bounds-check elimination
		}
		c = cs[i]
		switch c {
		case '"', '\\', '/', '\'':
			d.buf = append(d.buf, c)
		case 'b':
			d.buf = append(d.buf, '\b')
		case 'f':
			d.buf = append(d.buf, '\f')
		case 'n':
			d.buf = append(d.buf, '\n')
		case 'r':
			d.buf = append(d.buf, '\r')
		case 't':
			d.buf = append(d.buf, '\t')
		case 'u':
			i = d.appendStringAsBytesSlashU(cs, i)
		default:
			d.d.errorf("unsupported escaped value: %c", c)
		}
		i++
		cursor = i
	}
	if len(cs) > 0 {
		if len(d.buf) > 0 && cursor < uint(len(cs)) {
			d.buf = append(d.buf, cs[cursor:i]...)
		} else {
			// if bytes, just return the cs got from readUntil.
			// do not do it for io, especially bufio, as the buffer is needed for other things
			cs = cs[:i]
			if d.d.bytes {
				return cs
			}
			d.buf = d.d.blist.check(d.buf, len(cs))
			copy(d.buf, cs)
		}
	}
	return d.buf
}

func (d *jsonDecDriver) appendStringAsBytesSlashU(cs []byte, i uint) uint {
	var r rune
	var rr uint32
	var j uint
	var c byte
	if uint(len(cs)) < i+4 {
		d.d.errorf("need at least 4 more bytes for unicode sequence")
		return 0 // bounds-check elimination
	}
	for _, c = range cs[i+1 : i+5] { // bounds-check-elimination
		// best to use explicit if-else
		// - not a table, etc which involve memory loads, array lookup with bounds checks, etc
		if c >= '0' && c <= '9' {
			rr = rr*16 + uint32(c-jsonU4Chk2)
		} else if c >= 'a' && c <= 'f' {
			rr = rr*16 + uint32(c-jsonU4Chk1)
		} else if c >= 'A' && c <= 'F' {
			rr = rr*16 + uint32(c-jsonU4Chk0)
		} else {
			r = unicode.ReplacementChar
			i += 4
			goto encode_rune
		}
	}
	r = rune(rr)
	i += 4
	if utf16.IsSurrogate(r) {
		if len(cs) >= int(i+6) {
			var cx = cs[i+1:][:6:6] // [:6] affords bounds-check-elimination
			//var cx [6]byte
			//copy(cx[:], cs[i+1:])
			if cx[0] == '\\' && cx[1] == 'u' {
				i += 2
				var rr1 uint32
				for j = 2; j < 6; j++ {
					c = cx[j]
					if c >= '0' && c <= '9' {
						rr = rr*16 + uint32(c-jsonU4Chk2)
					} else if c >= 'a' && c <= 'f' {
						rr = rr*16 + uint32(c-jsonU4Chk1)
					} else if c >= 'A' && c <= 'F' {
						rr = rr*16 + uint32(c-jsonU4Chk0)
					} else {
						r = unicode.ReplacementChar
						i += 4
						goto encode_rune
					}
				}
				r = utf16.DecodeRune(r, rune(rr1))
				i += 4
				goto encode_rune
			}
		}
		r = unicode.ReplacementChar
	}
encode_rune:
	w2 := utf8.EncodeRune(d.bstr[:], r)
	d.buf = append(d.buf, d.bstr[:w2]...)
	return i
}

func (d *jsonDecDriver) nakedNum(z *decNaked, bs []byte) (err error) {
	const cutoff = uint64(1 << uint(64-1))

	var n uint64
	var neg, badsyntax, overflow bool

	if len(bs) == 0 {
		if d.h.PreferFloat {
			z.v = valueTypeFloat
			z.f = 0
		} else if d.h.SignedInteger {
			z.v = valueTypeInt
			z.i = 0
		} else {
			z.v = valueTypeUint
			z.u = 0
		}
		return
	}
	if d.h.PreferFloat {
		goto F
	}
	n, neg, badsyntax, overflow = jsonParseInteger(bs)
	if badsyntax || overflow {
		goto F
	}
	if neg {
		if n > cutoff {
			goto F
		}
		z.v = valueTypeInt
		z.i = -(int64(n))
	} else if d.h.SignedInteger {
		if n >= cutoff {
			goto F
		}
		z.v = valueTypeInt
		z.i = int64(n)
	} else {
		z.v = valueTypeUint
		z.u = n
	}
	return
F:
	z.v = valueTypeFloat
	z.f, err = parseFloat64(bs)
	return
}

func (d *jsonDecDriver) sliceToString(bs []byte) string {
	if d.d.is != nil && (jsonAlwaysReturnInternString || d.d.c == containerMapKey) {
		return d.d.string(bs)
	}
	return string(bs)
}

func (d *jsonDecDriver) DecodeNaked() {
	z := d.d.naked()

	d.advance()
	var bs []byte
	switch d.tok {
	case 'n':
		d.readLit4Null()
		z.v = valueTypeNil
	case 'f':
		d.readLit4False()
		z.v = valueTypeBool
		z.b = false
	case 't':
		d.readLit4True()
		z.v = valueTypeBool
		z.b = true
	case '{':
		z.v = valueTypeMap // don't consume. kInterfaceNaked will call ReadMapStart
	case '[':
		z.v = valueTypeArray // don't consume. kInterfaceNaked will call ReadArrayStart
	case '"':
		// if a string, and MapKeyAsString, then try to decode it as a nil, bool or number first
		bs = d.appendStringAsBytes()
		if len(bs) > 0 && d.d.c == containerMapKey && d.h.MapKeyAsString {
			if bytes.Equal(bs, jsonLiteralNull) {
				z.v = valueTypeNil
			} else if bytes.Equal(bs, jsonLiteralTrue) {
				z.v = valueTypeBool
				z.b = true
			} else if bytes.Equal(bs, jsonLiteralFalse) {
				z.v = valueTypeBool
				z.b = false
			} else {
				// check if a number: float, int or uint
				if err := d.nakedNum(z, bs); err != nil {
					z.v = valueTypeString
					z.s = d.sliceToString(bs)
				}
			}
		} else {
			z.v = valueTypeString
			z.s = d.sliceToString(bs)
		}
	default: // number
		bs = d.decNumBytes()
		if len(bs) == 0 {
			d.d.errorf("decode number from empty string")
			return
		}
		if err := d.nakedNum(z, bs); err != nil {
			d.d.errorf("decode number from %s: %v", bs, err)
			return
		}
	}
}

//----------------------

// JsonHandle is a handle for JSON encoding format.
//
// Json is comprehensively supported:
//    - decodes numbers into interface{} as int, uint or float64
//      based on how the number looks and some config parameters e.g. PreferFloat, SignedInt, etc.
//    - decode integers from float formatted numbers e.g. 1.27e+8
//    - decode any json value (numbers, bool, etc) from quoted strings
//    - configurable way to encode/decode []byte .
//      by default, encodes and decodes []byte using base64 Std Encoding
//    - UTF-8 support for encoding and decoding
//
// It has better performance than the json library in the standard library,
// by leveraging the performance improvements of the codec library.
//
// In addition, it doesn't read more bytes than necessary during a decode, which allows
// reading multiple values from a stream containing json and non-json content.
// For example, a user can read a json value, then a cbor value, then a msgpack value,
// all from the same stream in sequence.
//
// Note that, when decoding quoted strings, invalid UTF-8 or invalid UTF-16 surrogate pairs are
// not treated as an error. Instead, they are replaced by the Unicode replacement character U+FFFD.
type JsonHandle struct {
	textEncodingType
	BasicHandle

	// Indent indicates how a value is encoded.
	//   - If positive, indent by that number of spaces.
	//   - If negative, indent by that number of tabs.
	Indent int8

	// IntegerAsString controls how integers (signed and unsigned) are encoded.
	//
	// Per the JSON Spec, JSON numbers are 64-bit floating point numbers.
	// Consequently, integers > 2^53 cannot be represented as a JSON number without losing precision.
	// This can be mitigated by configuring how to encode integers.
	//
	// IntegerAsString interpretes the following values:
	//   - if 'L', then encode integers > 2^53 as a json string.
	//   - if 'A', then encode all integers as a json string
	//             containing the exact integer representation as a decimal.
	//   - else    encode all integers as a json number (default)
	IntegerAsString byte

	// HTMLCharsAsIs controls how to encode some special characters to html: < > &
	//
	// By default, we encode them as \uXXX
	// to prevent security holes when served from some browsers.
	HTMLCharsAsIs bool

	// PreferFloat says that we will default to decoding a number as a float.
	// If not set, we will examine the characters of the number and decode as an
	// integer type if it doesn't have any of the characters [.eE].
	PreferFloat bool

	// TermWhitespace says that we add a whitespace character
	// at the end of an encoding.
	//
	// The whitespace is important, especially if using numbers in a context
	// where multiple items are written to a stream.
	TermWhitespace bool

	// MapKeyAsString says to encode all map keys as strings.
	//
	// Use this to enforce strict json output.
	// The only caveat is that nil value is ALWAYS written as null (never as "null")
	MapKeyAsString bool

	// _ uint64 // padding (cache line)

	// Note: below, we store hardly-used items
	// e.g. RawBytesExt (which is already cached in the (en|de)cDriver).

	// RawBytesExt, if configured, is used to encode and decode raw bytes in a custom way.
	// If not configured, raw bytes are encoded to/from base64 text.
	RawBytesExt InterfaceExt

	_ [5]uint64 // padding (cache line)
}

// Name returns the name of the handle: json
func (h *JsonHandle) Name() string { return "json" }

// func (h *JsonHandle) hasElemSeparators() bool { return true }
func (h *JsonHandle) typical() bool {
	return h.Indent == 0 && !h.MapKeyAsString && h.IntegerAsString != 'A' && h.IntegerAsString != 'L'
}

func (h *JsonHandle) newEncDriver() encDriver {
	var e = &jsonEncDriver{h: h}
	e.e.e = e
	e.e.js = true
	e.e.init(h)
	e.reset()
	return e
}

func (h *JsonHandle) newDecDriver() decDriver {
	var d = &jsonDecDriver{h: h}
	d.d.d = d
	d.d.js = true
	d.d.jsms = h.MapKeyAsString
	d.d.init(h)
	d.reset()
	return d
}

func (e *jsonEncDriver) reset() {
	// (htmlasis && jsonCharSafeSet.isset(b)) || jsonCharHtmlSafeSet.isset(b)
	e.typical = e.h.typical()
	if e.h.HTMLCharsAsIs {
		e.s = &jsonCharSafeSet
	} else {
		e.s = &jsonCharHtmlSafeSet
	}
	e.se.InterfaceExt = e.h.RawBytesExt
	e.d, e.dl, e.di = false, 0, 0
	if e.h.Indent != 0 {
		e.d = true
		e.di = int8(e.h.Indent)
	}
	e.ks = e.h.MapKeyAsString
	e.is = e.h.IntegerAsString
}

func (d *jsonDecDriver) reset() {
	d.se.InterfaceExt = d.h.RawBytesExt
	d.buf = d.d.blist.check(d.buf, 256)[:0]
	d.tok = 0
	d.fnil = false
}

func (d *jsonDecDriver) atEndOfDecode() {}

// jsonFloatStrconvFmtPrec ...
//
// ensure that every float has an 'e' or '.' in it,/ for easy differentiation from integers.
// this is better/faster than checking if  encoded value has [e.] and appending if needed.

// func jsonFloatStrconvFmtPrec(f float64, bits32 bool) (fmt byte, prec int) {
// 	fmt = 'f'
// 	prec = -1
// 	var abs = math.Abs(f)
// 	if abs == 0 || abs == 1 {
// 		prec = 1
// 	} else if !bits32 && (abs < 1e-6 || abs >= 1e21) ||
// 		bits32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
// 		fmt = 'e'
// 	} else if _, frac := math.Modf(abs); frac == 0 {
// 		// ensure that floats have a .0 at the end, for easy identification as floats
// 		prec = 1
// 	}
// 	return
// }

func jsonFloatStrconvFmtPrec64(f float64) (fmt byte, prec int8) {
	fmt = 'f'
	prec = -1
	var abs = math.Abs(f)
	if abs == 0 || abs == 1 {
		prec = 1
	} else if abs < 1e-6 || abs >= 1e21 {
		fmt = 'e'
	} else if noFrac64(abs) { // _, frac := math.Modf(abs); frac == 0 {
		prec = 1
	}
	return
}

func jsonFloatStrconvFmtPrec32(f float32) (fmt byte, prec int8) {
	fmt = 'f'
	prec = -1
	var abs = abs32(f)
	if abs == 0 || abs == 1 {
		prec = 1
	} else if abs < 1e-6 || abs >= 1e21 {
		fmt = 'e'
	} else if noFrac32(abs) { // _, frac := math.Modf(abs); frac == 0 {
		prec = 1
	}
	return
}

// custom-fitted version of strconv.Parse(Ui|I)nt.
// Also ensures we don't have to search for .eE to determine if a float or not.
// Note: s CANNOT be a zero-length slice.
func jsonParseInteger(s []byte) (n uint64, neg, badSyntax, overflow bool) {
	const maxUint64 = (1<<64 - 1)
	const cutoff = maxUint64/10 + 1

	if len(s) == 0 { // bounds-check-elimination
		// treat empty string as zero value
		// badSyntax = true
		return
	}
	switch s[0] {
	case '+':
		s = s[1:]
	case '-':
		s = s[1:]
		neg = true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			badSyntax = true
			return
		}
		// unsigned integers don't overflow well on multiplication, so check cutoff here
		// e.g. (maxUint64-5)*10 doesn't overflow well ...
		if n >= cutoff {
			overflow = true
			return
		}
		n *= 10
		n1 := n + uint64(c-'0')
		if n1 < n || n1 > maxUint64 {
			overflow = true
			return
		}
		n = n1
	}
	return
}

var _ decDriverContainerTracker = (*jsonDecDriver)(nil)
var _ encDriverContainerTracker = (*jsonEncDriver)(nil)
var _ decDriver = (*jsonDecDriver)(nil)

var _ encDriver = (*jsonEncDriver)(nil)

// ----------------

/*
type jsonEncDriverTypical jsonEncDriver

func (e *jsonEncDriverTypical) WriteArrayStart(length int) {
	e.e.encWr.writen1('[')
}

func (e *jsonEncDriverTypical) WriteArrayElem() {
	if e.e.c != containerArrayStart {
		e.e.encWr.writen1(',')
	}
}

func (e *jsonEncDriverTypical) WriteArrayEnd() {
	e.e.encWr.writen1(']')
}

func (e *jsonEncDriverTypical) WriteMapStart(length int) {
	e.e.encWr.writen1('{')
}

func (e *jsonEncDriverTypical) WriteMapElemKey() {
	if e.e.c != containerMapStart {
		e.e.encWr.writen1(',')
	}
}

func (e *jsonEncDriverTypical) WriteMapElemValue() {
	e.e.encWr.writen1(':')
}

func (e *jsonEncDriverTypical) WriteMapEnd() {
	e.e.encWr.writen1('}')
}

func (e *jsonEncDriverTypical) EncodeBool(b bool) {
	if b {
		// e.e.encWr.writeb(jsonLiteralTrue)
		e.e.encWr.writen([rwNLen]byte{'t', 'r', 'u', 'e'}, 4)
	} else {
		// e.e.encWr.writeb(jsonLiteralFalse)
		e.e.encWr.writen([rwNLen]byte{'f', 'a', 'l', 's', 'e'}, 5)
	}
}

func (e *jsonEncDriverTypical) EncodeInt(v int64) {
	e.e.encWr.writeb(strconv.AppendInt(e.b[:0], v, 10))
}

func (e *jsonEncDriverTypical) EncodeUint(v uint64) {
	e.e.encWr.writeb(strconv.AppendUint(e.b[:0], v, 10))
}

func (e *jsonEncDriverTypical) EncodeFloat64(f float64) {
	fmt, prec := jsonFloatStrconvFmtPrec64(f)
	e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], f, fmt, int(prec), 64))
	// e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], f, jsonFloatStrconvFmtPrec64(f), 64))
}

func (e *jsonEncDriverTypical) EncodeFloat32(f float32) {
	fmt, prec := jsonFloatStrconvFmtPrec32(f)
	e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], float64(f), fmt, int(prec), 32))
}

// func (e *jsonEncDriverTypical) encodeFloat(f float64, bitsize uint8) {
// 	fmt, prec := jsonFloatStrconvFmtPrec(f, bitsize == 32)
// 	e.e.encWr.writeb(strconv.AppendFloat(e.b[:0], f, fmt, prec, int(bitsize)))
// }

// func (e *jsonEncDriverTypical) atEndOfEncode() {
// 	if e.tw {
// 		e.e.encWr.writen1(' ')
// 	}
// }

*/
