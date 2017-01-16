// Package qr can be used to create QR barcodes.
package qr

import (
	"image"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/utils"
)

type encodeFn func(content string, eccLevel ErrorCorrectionLevel) (*utils.BitList, *versionInfo, error)

// Encoding mode for QR Codes.
type Encoding byte

const (
	// Auto will choose ths best matching encoding
	Auto Encoding = iota
	// Numeric encoding only encodes numbers [0-9]
	Numeric
	// AlphaNumeric encoding only encodes uppercase letters, numbers and  [Space], $, %, *, +, -, ., /, :
	AlphaNumeric
	// Unicode encoding encodes the string as utf-8
	Unicode
	// only for testing purpose
	unknownEncoding
)

func (e Encoding) getEncoder() encodeFn {
	switch e {
	case Auto:
		return encodeAuto
	case Numeric:
		return encodeNumeric
	case AlphaNumeric:
		return encodeAlphaNumeric
	case Unicode:
		return encodeUnicode
	}
	return nil
}

func (e Encoding) String() string {
	switch e {
	case Auto:
		return "Auto"
	case Numeric:
		return "Numeric"
	case AlphaNumeric:
		return "AlphaNumeric"
	case Unicode:
		return "Unicode"
	}
	return ""
}

// Encode returns a QR barcode with the given content, error correction level and uses the given encoding
func Encode(content string, level ErrorCorrectionLevel, mode Encoding) (barcode.Barcode, error) {
	bits, vi, err := mode.getEncoder()(content, level)
	if err != nil {
		return nil, err
	}

	blocks := splitToBlocks(bits.IterateBytes(), vi)
	data := blocks.interleave(vi)
	result := render(data, vi)
	result.content = content
	return result, nil
}

func render(data []byte, vi *versionInfo) *qrcode {
	dim := vi.modulWidth()
	results := make([]*qrcode, 8)
	for i := 0; i < 8; i++ {
		results[i] = newBarcode(dim)
	}

	occupied := newBarcode(dim)

	setAll := func(x int, y int, val bool) {
		occupied.Set(x, y, true)
		for i := 0; i < 8; i++ {
			results[i].Set(x, y, val)
		}
	}

	drawFinderPatterns(vi, setAll)
	drawAlignmentPatterns(occupied, vi, setAll)

	//Timing Pattern:
	var i int
	for i = 0; i < dim; i++ {
		if !occupied.Get(i, 6) {
			setAll(i, 6, i%2 == 0)
		}
		if !occupied.Get(6, i) {
			setAll(6, i, i%2 == 0)
		}
	}
	// Dark Module
	setAll(8, dim-8, true)

	drawVersionInfo(vi, setAll)
	drawFormatInfo(vi, -1, occupied.Set)
	for i := 0; i < 8; i++ {
		drawFormatInfo(vi, i, results[i].Set)
	}

	// Write the data
	var curBitNo int

	for pos := range iterateModules(occupied) {
		var curBit bool
		if curBitNo < len(data)*8 {
			curBit = ((data[curBitNo/8] >> uint(7-(curBitNo%8))) & 1) == 1
		} else {
			curBit = false
		}

		for i := 0; i < 8; i++ {
			setMasked(pos.X, pos.Y, curBit, i, results[i].Set)
		}
		curBitNo++
	}

	lowestPenalty := ^uint(0)
	lowestPenaltyIdx := -1
	for i := 0; i < 8; i++ {
		p := results[i].calcPenalty()
		if p < lowestPenalty {
			lowestPenalty = p
			lowestPenaltyIdx = i
		}
	}
	return results[lowestPenaltyIdx]
}

func setMasked(x, y int, val bool, mask int, set func(int, int, bool)) {
	switch mask {
	case 0:
		val = val != (((y + x) % 2) == 0)
		break
	case 1:
		val = val != ((y % 2) == 0)
		break
	case 2:
		val = val != ((x % 3) == 0)
		break
	case 3:
		val = val != (((y + x) % 3) == 0)
		break
	case 4:
		val = val != (((y/2 + x/3) % 2) == 0)
		break
	case 5:
		val = val != (((y*x)%2)+((y*x)%3) == 0)
		break
	case 6:
		val = val != ((((y*x)%2)+((y*x)%3))%2 == 0)
		break
	case 7:
		val = val != ((((y+x)%2)+((y*x)%3))%2 == 0)
	}
	set(x, y, val)
}

func iterateModules(occupied *qrcode) <-chan image.Point {
	result := make(chan image.Point)
	allPoints := make(chan image.Point)
	go func() {
		curX := occupied.dimension - 1
		curY := occupied.dimension - 1
		isUpward := true

		for true {
			if isUpward {
				allPoints <- image.Pt(curX, curY)
				allPoints <- image.Pt(curX-1, curY)
				curY--
				if curY < 0 {
					curY = 0
					curX -= 2
					if curX == 6 {
						curX--
					}
					if curX < 0 {
						break
					}
					isUpward = false
				}
			} else {
				allPoints <- image.Pt(curX, curY)
				allPoints <- image.Pt(curX-1, curY)
				curY++
				if curY >= occupied.dimension {
					curY = occupied.dimension - 1
					curX -= 2
					if curX == 6 {
						curX--
					}
					isUpward = true
					if curX < 0 {
						break
					}
				}
			}
		}

		close(allPoints)
	}()
	go func() {
		for pt := range allPoints {
			if !occupied.Get(pt.X, pt.Y) {
				result <- pt
			}
		}
		close(result)
	}()
	return result
}

func drawFinderPatterns(vi *versionInfo, set func(int, int, bool)) {
	dim := vi.modulWidth()
	drawPattern := func(xoff int, yoff int) {
		for x := -1; x < 8; x++ {
			for y := -1; y < 8; y++ {
				val := (x == 0 || x == 6 || y == 0 || y == 6 || (x > 1 && x < 5 && y > 1 && y < 5)) && (x <= 6 && y <= 6 && x >= 0 && y >= 0)

				if x+xoff >= 0 && x+xoff < dim && y+yoff >= 0 && y+yoff < dim {
					set(x+xoff, y+yoff, val)
				}
			}
		}
	}
	drawPattern(0, 0)
	drawPattern(0, dim-7)
	drawPattern(dim-7, 0)
}

func drawAlignmentPatterns(occupied *qrcode, vi *versionInfo, set func(int, int, bool)) {
	drawPattern := func(xoff int, yoff int) {
		for x := -2; x <= 2; x++ {
			for y := -2; y <= 2; y++ {
				val := x == -2 || x == 2 || y == -2 || y == 2 || (x == 0 && y == 0)
				set(x+xoff, y+yoff, val)
			}
		}
	}
	positions := vi.alignmentPatternPlacements()

	for _, x := range positions {
		for _, y := range positions {
			if occupied.Get(x, y) {
				continue
			}
			drawPattern(x, y)
		}
	}
}

var formatInfos = map[ErrorCorrectionLevel]map[int][]bool{
	L: {
		0: []bool{true, true, true, false, true, true, true, true, true, false, false, false, true, false, false},
		1: []bool{true, true, true, false, false, true, false, true, true, true, true, false, false, true, true},
		2: []bool{true, true, true, true, true, false, true, true, false, true, false, true, false, true, false},
		3: []bool{true, true, true, true, false, false, false, true, false, false, true, true, true, false, true},
		4: []bool{true, true, false, false, true, true, false, false, false, true, false, true, true, true, true},
		5: []bool{true, true, false, false, false, true, true, false, false, false, true, true, false, false, false},
		6: []bool{true, true, false, true, true, false, false, false, true, false, false, false, false, false, true},
		7: []bool{true, true, false, true, false, false, true, false, true, true, true, false, true, true, false},
	},
	M: {
		0: []bool{true, false, true, false, true, false, false, false, false, false, true, false, false, true, false},
		1: []bool{true, false, true, false, false, false, true, false, false, true, false, false, true, false, true},
		2: []bool{true, false, true, true, true, true, false, false, true, true, true, true, true, false, false},
		3: []bool{true, false, true, true, false, true, true, false, true, false, false, true, false, true, true},
		4: []bool{true, false, false, false, true, false, true, true, true, true, true, true, false, false, true},
		5: []bool{true, false, false, false, false, false, false, true, true, false, false, true, true, true, false},
		6: []bool{true, false, false, true, true, true, true, true, false, false, true, false, true, true, true},
		7: []bool{true, false, false, true, false, true, false, true, false, true, false, false, false, false, false},
	},
	Q: {
		0: []bool{false, true, true, false, true, false, true, false, true, false, true, true, true, true, true},
		1: []bool{false, true, true, false, false, false, false, false, true, true, false, true, false, false, false},
		2: []bool{false, true, true, true, true, true, true, false, false, true, true, false, false, false, true},
		3: []bool{false, true, true, true, false, true, false, false, false, false, false, false, true, true, false},
		4: []bool{false, true, false, false, true, false, false, true, false, true, true, false, true, false, false},
		5: []bool{false, true, false, false, false, false, true, true, false, false, false, false, false, true, true},
		6: []bool{false, true, false, true, true, true, false, true, true, false, true, true, false, true, false},
		7: []bool{false, true, false, true, false, true, true, true, true, true, false, true, true, false, true},
	},
	H: {
		0: []bool{false, false, true, false, true, true, false, true, false, false, false, true, false, false, true},
		1: []bool{false, false, true, false, false, true, true, true, false, true, true, true, true, true, false},
		2: []bool{false, false, true, true, true, false, false, true, true, true, false, false, true, true, true},
		3: []bool{false, false, true, true, false, false, true, true, true, false, true, false, false, false, false},
		4: []bool{false, false, false, false, true, true, true, false, true, true, false, false, false, true, false},
		5: []bool{false, false, false, false, false, true, false, false, true, false, true, false, true, false, true},
		6: []bool{false, false, false, true, true, false, true, false, false, false, false, true, true, false, false},
		7: []bool{false, false, false, true, false, false, false, false, false, true, true, true, false, true, true},
	},
}

func drawFormatInfo(vi *versionInfo, usedMask int, set func(int, int, bool)) {
	var formatInfo []bool

	if usedMask == -1 {
		formatInfo = []bool{true, true, true, true, true, true, true, true, true, true, true, true, true, true, true} // Set all to true cause -1 --> occupied mask.
	} else {
		formatInfo = formatInfos[vi.Level][usedMask]
	}

	if len(formatInfo) == 15 {
		dim := vi.modulWidth()
		set(0, 8, formatInfo[0])
		set(1, 8, formatInfo[1])
		set(2, 8, formatInfo[2])
		set(3, 8, formatInfo[3])
		set(4, 8, formatInfo[4])
		set(5, 8, formatInfo[5])
		set(7, 8, formatInfo[6])
		set(8, 8, formatInfo[7])
		set(8, 7, formatInfo[8])
		set(8, 5, formatInfo[9])
		set(8, 4, formatInfo[10])
		set(8, 3, formatInfo[11])
		set(8, 2, formatInfo[12])
		set(8, 1, formatInfo[13])
		set(8, 0, formatInfo[14])

		set(8, dim-1, formatInfo[0])
		set(8, dim-2, formatInfo[1])
		set(8, dim-3, formatInfo[2])
		set(8, dim-4, formatInfo[3])
		set(8, dim-5, formatInfo[4])
		set(8, dim-6, formatInfo[5])
		set(8, dim-7, formatInfo[6])
		set(dim-8, 8, formatInfo[7])
		set(dim-7, 8, formatInfo[8])
		set(dim-6, 8, formatInfo[9])
		set(dim-5, 8, formatInfo[10])
		set(dim-4, 8, formatInfo[11])
		set(dim-3, 8, formatInfo[12])
		set(dim-2, 8, formatInfo[13])
		set(dim-1, 8, formatInfo[14])
	}
}

var versionInfoBitsByVersion = map[byte][]bool{
	7:  []bool{false, false, false, true, true, true, true, true, false, false, true, false, false, true, false, true, false, false},
	8:  []bool{false, false, true, false, false, false, false, true, false, true, true, false, true, true, true, true, false, false},
	9:  []bool{false, false, true, false, false, true, true, false, true, false, true, false, false, true, true, false, false, true},
	10: []bool{false, false, true, false, true, false, false, true, false, false, true, true, false, true, false, false, true, true},
	11: []bool{false, false, true, false, true, true, true, false, true, true, true, true, true, true, false, true, true, false},
	12: []bool{false, false, true, true, false, false, false, true, true, true, false, true, true, false, false, false, true, false},
	13: []bool{false, false, true, true, false, true, true, false, false, false, false, true, false, false, false, true, true, true},
	14: []bool{false, false, true, true, true, false, false, true, true, false, false, false, false, false, true, true, false, true},
	15: []bool{false, false, true, true, true, true, true, false, false, true, false, false, true, false, true, false, false, false},
	16: []bool{false, true, false, false, false, false, true, false, true, true, false, true, true, true, true, false, false, false},
	17: []bool{false, true, false, false, false, true, false, true, false, false, false, true, false, true, true, true, false, true},
	18: []bool{false, true, false, false, true, false, true, false, true, false, false, false, false, true, false, true, true, true},
	19: []bool{false, true, false, false, true, true, false, true, false, true, false, false, true, true, false, false, true, false},
	20: []bool{false, true, false, true, false, false, true, false, false, true, true, false, true, false, false, true, true, false},
	21: []bool{false, true, false, true, false, true, false, true, true, false, true, false, false, false, false, false, true, true},
	22: []bool{false, true, false, true, true, false, true, false, false, false, true, true, false, false, true, false, false, true},
	23: []bool{false, true, false, true, true, true, false, true, true, true, true, true, true, false, true, true, false, false},
	24: []bool{false, true, true, false, false, false, true, true, true, false, true, true, false, false, false, true, false, false},
	25: []bool{false, true, true, false, false, true, false, false, false, true, true, true, true, false, false, false, false, true},
	26: []bool{false, true, true, false, true, false, true, true, true, true, true, false, true, false, true, false, true, true},
	27: []bool{false, true, true, false, true, true, false, false, false, false, true, false, false, false, true, true, true, false},
	28: []bool{false, true, true, true, false, false, true, true, false, false, false, false, false, true, true, false, true, false},
	29: []bool{false, true, true, true, false, true, false, false, true, true, false, false, true, true, true, true, true, true},
	30: []bool{false, true, true, true, true, false, true, true, false, true, false, true, true, true, false, true, false, true},
	31: []bool{false, true, true, true, true, true, false, false, true, false, false, true, false, true, false, false, false, false},
	32: []bool{true, false, false, false, false, false, true, false, false, true, true, true, false, true, false, true, false, true},
	33: []bool{true, false, false, false, false, true, false, true, true, false, true, true, true, true, false, false, false, false},
	34: []bool{true, false, false, false, true, false, true, false, false, false, true, false, true, true, true, false, true, false},
	35: []bool{true, false, false, false, true, true, false, true, true, true, true, false, false, true, true, true, true, true},
	36: []bool{true, false, false, true, false, false, true, false, true, true, false, false, false, false, true, false, true, true},
	37: []bool{true, false, false, true, false, true, false, true, false, false, false, false, true, false, true, true, true, false},
	38: []bool{true, false, false, true, true, false, true, false, true, false, false, true, true, false, false, true, false, false},
	39: []bool{true, false, false, true, true, true, false, true, false, true, false, true, false, false, false, false, false, true},
	40: []bool{true, false, true, false, false, false, true, true, false, false, false, true, true, false, true, false, false, true},
}

func drawVersionInfo(vi *versionInfo, set func(int, int, bool)) {
	versionInfoBits, ok := versionInfoBitsByVersion[vi.Version]

	if ok && len(versionInfoBits) > 0 {
		for i := 0; i < len(versionInfoBits); i++ {
			x := (vi.modulWidth() - 11) + i%3
			y := i / 3
			set(x, y, versionInfoBits[len(versionInfoBits)-i-1])
			set(y, x, versionInfoBits[len(versionInfoBits)-i-1])
		}
	}

}

func addPaddingAndTerminator(bl *utils.BitList, vi *versionInfo) {
	for i := 0; i < 4 && bl.Len() < vi.totalDataBytes()*8; i++ {
		bl.AddBit(false)
	}

	for bl.Len()%8 != 0 {
		bl.AddBit(false)
	}

	for i := 0; bl.Len() < vi.totalDataBytes()*8; i++ {
		if i%2 == 0 {
			bl.AddByte(236)
		} else {
			bl.AddByte(17)
		}
	}
}
