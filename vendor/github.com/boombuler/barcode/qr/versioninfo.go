package qr

import "math"

// ErrorCorrectionLevel indicates the amount of "backup data" stored in the QR code
type ErrorCorrectionLevel byte

const (
	// L recovers 7% of data
	L ErrorCorrectionLevel = iota
	// M recovers 15% of data
	M
	// Q recovers 25% of data
	Q
	// H recovers 30% of data
	H
)

func (ecl ErrorCorrectionLevel) String() string {
	switch ecl {
	case L:
		return "L"
	case M:
		return "M"
	case Q:
		return "Q"
	case H:
		return "H"
	}
	return "unknown"
}

type encodingMode byte

const (
	numericMode      encodingMode = 1
	alphaNumericMode encodingMode = 2
	byteMode         encodingMode = 4
	kanjiMode        encodingMode = 8
)

type versionInfo struct {
	Version                          byte
	Level                            ErrorCorrectionLevel
	ErrorCorrectionCodewordsPerBlock byte
	NumberOfBlocksInGroup1           byte
	DataCodeWordsPerBlockInGroup1    byte
	NumberOfBlocksInGroup2           byte
	DataCodeWordsPerBlockInGroup2    byte
}

var versionInfos = []*versionInfo{
	&versionInfo{1, L, 7, 1, 19, 0, 0},
	&versionInfo{1, M, 10, 1, 16, 0, 0},
	&versionInfo{1, Q, 13, 1, 13, 0, 0},
	&versionInfo{1, H, 17, 1, 9, 0, 0},
	&versionInfo{2, L, 10, 1, 34, 0, 0},
	&versionInfo{2, M, 16, 1, 28, 0, 0},
	&versionInfo{2, Q, 22, 1, 22, 0, 0},
	&versionInfo{2, H, 28, 1, 16, 0, 0},
	&versionInfo{3, L, 15, 1, 55, 0, 0},
	&versionInfo{3, M, 26, 1, 44, 0, 0},
	&versionInfo{3, Q, 18, 2, 17, 0, 0},
	&versionInfo{3, H, 22, 2, 13, 0, 0},
	&versionInfo{4, L, 20, 1, 80, 0, 0},
	&versionInfo{4, M, 18, 2, 32, 0, 0},
	&versionInfo{4, Q, 26, 2, 24, 0, 0},
	&versionInfo{4, H, 16, 4, 9, 0, 0},
	&versionInfo{5, L, 26, 1, 108, 0, 0},
	&versionInfo{5, M, 24, 2, 43, 0, 0},
	&versionInfo{5, Q, 18, 2, 15, 2, 16},
	&versionInfo{5, H, 22, 2, 11, 2, 12},
	&versionInfo{6, L, 18, 2, 68, 0, 0},
	&versionInfo{6, M, 16, 4, 27, 0, 0},
	&versionInfo{6, Q, 24, 4, 19, 0, 0},
	&versionInfo{6, H, 28, 4, 15, 0, 0},
	&versionInfo{7, L, 20, 2, 78, 0, 0},
	&versionInfo{7, M, 18, 4, 31, 0, 0},
	&versionInfo{7, Q, 18, 2, 14, 4, 15},
	&versionInfo{7, H, 26, 4, 13, 1, 14},
	&versionInfo{8, L, 24, 2, 97, 0, 0},
	&versionInfo{8, M, 22, 2, 38, 2, 39},
	&versionInfo{8, Q, 22, 4, 18, 2, 19},
	&versionInfo{8, H, 26, 4, 14, 2, 15},
	&versionInfo{9, L, 30, 2, 116, 0, 0},
	&versionInfo{9, M, 22, 3, 36, 2, 37},
	&versionInfo{9, Q, 20, 4, 16, 4, 17},
	&versionInfo{9, H, 24, 4, 12, 4, 13},
	&versionInfo{10, L, 18, 2, 68, 2, 69},
	&versionInfo{10, M, 26, 4, 43, 1, 44},
	&versionInfo{10, Q, 24, 6, 19, 2, 20},
	&versionInfo{10, H, 28, 6, 15, 2, 16},
	&versionInfo{11, L, 20, 4, 81, 0, 0},
	&versionInfo{11, M, 30, 1, 50, 4, 51},
	&versionInfo{11, Q, 28, 4, 22, 4, 23},
	&versionInfo{11, H, 24, 3, 12, 8, 13},
	&versionInfo{12, L, 24, 2, 92, 2, 93},
	&versionInfo{12, M, 22, 6, 36, 2, 37},
	&versionInfo{12, Q, 26, 4, 20, 6, 21},
	&versionInfo{12, H, 28, 7, 14, 4, 15},
	&versionInfo{13, L, 26, 4, 107, 0, 0},
	&versionInfo{13, M, 22, 8, 37, 1, 38},
	&versionInfo{13, Q, 24, 8, 20, 4, 21},
	&versionInfo{13, H, 22, 12, 11, 4, 12},
	&versionInfo{14, L, 30, 3, 115, 1, 116},
	&versionInfo{14, M, 24, 4, 40, 5, 41},
	&versionInfo{14, Q, 20, 11, 16, 5, 17},
	&versionInfo{14, H, 24, 11, 12, 5, 13},
	&versionInfo{15, L, 22, 5, 87, 1, 88},
	&versionInfo{15, M, 24, 5, 41, 5, 42},
	&versionInfo{15, Q, 30, 5, 24, 7, 25},
	&versionInfo{15, H, 24, 11, 12, 7, 13},
	&versionInfo{16, L, 24, 5, 98, 1, 99},
	&versionInfo{16, M, 28, 7, 45, 3, 46},
	&versionInfo{16, Q, 24, 15, 19, 2, 20},
	&versionInfo{16, H, 30, 3, 15, 13, 16},
	&versionInfo{17, L, 28, 1, 107, 5, 108},
	&versionInfo{17, M, 28, 10, 46, 1, 47},
	&versionInfo{17, Q, 28, 1, 22, 15, 23},
	&versionInfo{17, H, 28, 2, 14, 17, 15},
	&versionInfo{18, L, 30, 5, 120, 1, 121},
	&versionInfo{18, M, 26, 9, 43, 4, 44},
	&versionInfo{18, Q, 28, 17, 22, 1, 23},
	&versionInfo{18, H, 28, 2, 14, 19, 15},
	&versionInfo{19, L, 28, 3, 113, 4, 114},
	&versionInfo{19, M, 26, 3, 44, 11, 45},
	&versionInfo{19, Q, 26, 17, 21, 4, 22},
	&versionInfo{19, H, 26, 9, 13, 16, 14},
	&versionInfo{20, L, 28, 3, 107, 5, 108},
	&versionInfo{20, M, 26, 3, 41, 13, 42},
	&versionInfo{20, Q, 30, 15, 24, 5, 25},
	&versionInfo{20, H, 28, 15, 15, 10, 16},
	&versionInfo{21, L, 28, 4, 116, 4, 117},
	&versionInfo{21, M, 26, 17, 42, 0, 0},
	&versionInfo{21, Q, 28, 17, 22, 6, 23},
	&versionInfo{21, H, 30, 19, 16, 6, 17},
	&versionInfo{22, L, 28, 2, 111, 7, 112},
	&versionInfo{22, M, 28, 17, 46, 0, 0},
	&versionInfo{22, Q, 30, 7, 24, 16, 25},
	&versionInfo{22, H, 24, 34, 13, 0, 0},
	&versionInfo{23, L, 30, 4, 121, 5, 122},
	&versionInfo{23, M, 28, 4, 47, 14, 48},
	&versionInfo{23, Q, 30, 11, 24, 14, 25},
	&versionInfo{23, H, 30, 16, 15, 14, 16},
	&versionInfo{24, L, 30, 6, 117, 4, 118},
	&versionInfo{24, M, 28, 6, 45, 14, 46},
	&versionInfo{24, Q, 30, 11, 24, 16, 25},
	&versionInfo{24, H, 30, 30, 16, 2, 17},
	&versionInfo{25, L, 26, 8, 106, 4, 107},
	&versionInfo{25, M, 28, 8, 47, 13, 48},
	&versionInfo{25, Q, 30, 7, 24, 22, 25},
	&versionInfo{25, H, 30, 22, 15, 13, 16},
	&versionInfo{26, L, 28, 10, 114, 2, 115},
	&versionInfo{26, M, 28, 19, 46, 4, 47},
	&versionInfo{26, Q, 28, 28, 22, 6, 23},
	&versionInfo{26, H, 30, 33, 16, 4, 17},
	&versionInfo{27, L, 30, 8, 122, 4, 123},
	&versionInfo{27, M, 28, 22, 45, 3, 46},
	&versionInfo{27, Q, 30, 8, 23, 26, 24},
	&versionInfo{27, H, 30, 12, 15, 28, 16},
	&versionInfo{28, L, 30, 3, 117, 10, 118},
	&versionInfo{28, M, 28, 3, 45, 23, 46},
	&versionInfo{28, Q, 30, 4, 24, 31, 25},
	&versionInfo{28, H, 30, 11, 15, 31, 16},
	&versionInfo{29, L, 30, 7, 116, 7, 117},
	&versionInfo{29, M, 28, 21, 45, 7, 46},
	&versionInfo{29, Q, 30, 1, 23, 37, 24},
	&versionInfo{29, H, 30, 19, 15, 26, 16},
	&versionInfo{30, L, 30, 5, 115, 10, 116},
	&versionInfo{30, M, 28, 19, 47, 10, 48},
	&versionInfo{30, Q, 30, 15, 24, 25, 25},
	&versionInfo{30, H, 30, 23, 15, 25, 16},
	&versionInfo{31, L, 30, 13, 115, 3, 116},
	&versionInfo{31, M, 28, 2, 46, 29, 47},
	&versionInfo{31, Q, 30, 42, 24, 1, 25},
	&versionInfo{31, H, 30, 23, 15, 28, 16},
	&versionInfo{32, L, 30, 17, 115, 0, 0},
	&versionInfo{32, M, 28, 10, 46, 23, 47},
	&versionInfo{32, Q, 30, 10, 24, 35, 25},
	&versionInfo{32, H, 30, 19, 15, 35, 16},
	&versionInfo{33, L, 30, 17, 115, 1, 116},
	&versionInfo{33, M, 28, 14, 46, 21, 47},
	&versionInfo{33, Q, 30, 29, 24, 19, 25},
	&versionInfo{33, H, 30, 11, 15, 46, 16},
	&versionInfo{34, L, 30, 13, 115, 6, 116},
	&versionInfo{34, M, 28, 14, 46, 23, 47},
	&versionInfo{34, Q, 30, 44, 24, 7, 25},
	&versionInfo{34, H, 30, 59, 16, 1, 17},
	&versionInfo{35, L, 30, 12, 121, 7, 122},
	&versionInfo{35, M, 28, 12, 47, 26, 48},
	&versionInfo{35, Q, 30, 39, 24, 14, 25},
	&versionInfo{35, H, 30, 22, 15, 41, 16},
	&versionInfo{36, L, 30, 6, 121, 14, 122},
	&versionInfo{36, M, 28, 6, 47, 34, 48},
	&versionInfo{36, Q, 30, 46, 24, 10, 25},
	&versionInfo{36, H, 30, 2, 15, 64, 16},
	&versionInfo{37, L, 30, 17, 122, 4, 123},
	&versionInfo{37, M, 28, 29, 46, 14, 47},
	&versionInfo{37, Q, 30, 49, 24, 10, 25},
	&versionInfo{37, H, 30, 24, 15, 46, 16},
	&versionInfo{38, L, 30, 4, 122, 18, 123},
	&versionInfo{38, M, 28, 13, 46, 32, 47},
	&versionInfo{38, Q, 30, 48, 24, 14, 25},
	&versionInfo{38, H, 30, 42, 15, 32, 16},
	&versionInfo{39, L, 30, 20, 117, 4, 118},
	&versionInfo{39, M, 28, 40, 47, 7, 48},
	&versionInfo{39, Q, 30, 43, 24, 22, 25},
	&versionInfo{39, H, 30, 10, 15, 67, 16},
	&versionInfo{40, L, 30, 19, 118, 6, 119},
	&versionInfo{40, M, 28, 18, 47, 31, 48},
	&versionInfo{40, Q, 30, 34, 24, 34, 25},
	&versionInfo{40, H, 30, 20, 15, 61, 16},
}

func (vi *versionInfo) totalDataBytes() int {
	g1Data := int(vi.NumberOfBlocksInGroup1) * int(vi.DataCodeWordsPerBlockInGroup1)
	g2Data := int(vi.NumberOfBlocksInGroup2) * int(vi.DataCodeWordsPerBlockInGroup2)
	return (g1Data + g2Data)
}

func (vi *versionInfo) charCountBits(m encodingMode) byte {
	switch m {
	case numericMode:
		if vi.Version < 10 {
			return 10
		} else if vi.Version < 27 {
			return 12
		}
		return 14

	case alphaNumericMode:
		if vi.Version < 10 {
			return 9
		} else if vi.Version < 27 {
			return 11
		}
		return 13

	case byteMode:
		if vi.Version < 10 {
			return 8
		}
		return 16

	case kanjiMode:
		if vi.Version < 10 {
			return 8
		} else if vi.Version < 27 {
			return 10
		}
		return 12
	default:
		return 0
	}
}

func (vi *versionInfo) modulWidth() int {
	return ((int(vi.Version) - 1) * 4) + 21
}

func (vi *versionInfo) alignmentPatternPlacements() []int {
	if vi.Version == 1 {
		return make([]int, 0)
	}

	first := 6
	last := vi.modulWidth() - 7
	space := float64(last - first)
	count := int(math.Ceil(space/28)) + 1

	result := make([]int, count)
	result[0] = first
	result[len(result)-1] = last
	if count > 2 {
		step := int(math.Ceil(float64(last-first) / float64(count-1)))
		if step%2 == 1 {
			frac := float64(last-first) / float64(count-1)
			_, x := math.Modf(frac)
			if x >= 0.5 {
				frac = math.Ceil(frac)
			} else {
				frac = math.Floor(frac)
			}

			if int(frac)%2 == 0 {
				step--
			} else {
				step++
			}
		}

		for i := 1; i <= count-2; i++ {
			result[i] = last - (step * (count - 1 - i))
		}
	}

	return result
}

func findSmallestVersionInfo(ecl ErrorCorrectionLevel, mode encodingMode, dataBits int) *versionInfo {
	dataBits = dataBits + 4 // mode indicator
	for _, vi := range versionInfos {
		if vi.Level == ecl {
			if (vi.totalDataBytes() * 8) >= (dataBits + int(vi.charCountBits(mode))) {
				return vi
			}
		}
	}
	return nil
}
